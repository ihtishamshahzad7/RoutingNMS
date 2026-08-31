package mib

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

const maxUploadBytes = 10 << 20 // 10MB is generous for a text MIB file

// API backs GET/POST /api/v1/mibs (list / upload).
type API struct{ Repo Repository }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		mibs, err := a.Repo.List(ctx)
		if err != nil {
			http.Error(w, "failed to load mibs", http.StatusInternalServerError)
			return
		}
		writeJSON(w, mibs)

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		filename := r.URL.Query().Get("filename")
		var raw []byte
		var err error

		contentType := r.Header.Get("Content-Type")
		if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
			if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
				http.Error(w, "invalid multipart upload", http.StatusBadRequest)
				return
			}
			file, header, ferr := r.FormFile("file")
			if ferr != nil {
				http.Error(w, "missing file field", http.StatusBadRequest)
				return
			}
			defer file.Close()
			if filename == "" {
				filename = header.Filename
			}
			raw, err = io.ReadAll(file)
		} else {
			raw, err = io.ReadAll(r.Body)
		}
		if err != nil {
			http.Error(w, "failed to read upload", http.StatusBadRequest)
			return
		}
		if len(raw) == 0 {
			http.Error(w, "uploaded file is empty", http.StatusBadRequest)
			return
		}
		if filename == "" {
			filename = "upload.mib"
		}

		stored, err := a.Repo.Upload(ctx, filename, string(raw))
		if err != nil {
			http.Error(w, "failed to parse/store mib", http.StatusInternalServerError)
			return
		}
		writeJSON(w, stored)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// MIBAPI backs DELETE /api/v1/mibs/{id}.
type MIBAPI struct{ Repo Repository }

func (a MIBAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid mib id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := a.Repo.Delete(ctx, id); err != nil {
		http.Error(w, "failed to delete mib", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SearchAPI backs GET /api/v1/mibs/search?q=.
type SearchAPI struct{ Repo Repository }

func (a SearchAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	results, err := a.Repo.Search(ctx, r.URL.Query().Get("q"), 50)
	if err != nil {
		http.Error(w, "failed to search mibs", http.StatusInternalServerError)
		return
	}
	writeJSON(w, results)
}

// TestAPI backs POST /api/v1/mibs/test: fetch a real OID from a real
// device, resolving the returned OID (and, best-effort, the requested one)
// to a friendly name via whatever MIBs have been uploaded.
type TestAPI struct {
	Repo      Repository
	Devices   devices.Repository
	Collector snmp.Collector
}

type testResult struct {
	OID          string `json:"oid"`
	ResolvedName string `json:"resolvedName,omitempty"`
	Value        any    `json:"value"`
}

func (a TestAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		DeviceID string `json:"deviceId"`
		OID      string `json:"oid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.DeviceID == "" || in.OID == "" {
		http.Error(w, "deviceId and oid are required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	input, err := a.Devices.DiscoveryTarget(ctx, in.DeviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := snmp.Target{ID: input.Name, Address: input.Address, Port: input.SNMPPort, Credentials: input.SNMP, Timeout: input.Timeout, Retries: 1}

	values, err := a.Collector.Get(ctx, target, []string{in.OID})
	if err != nil {
		http.Error(w, "SNMP GET failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if len(values) == 0 {
		http.Error(w, "device returned no value for this OID", http.StatusBadGateway)
		return
	}

	name, _ := a.Repo.ResolveName(ctx, in.OID)
	writeJSON(w, testResult{OID: in.OID, ResolvedName: name, Value: values[0].Value})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
