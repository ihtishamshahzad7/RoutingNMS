package discovery

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// TargetsAPI backs GET/POST /api/v1/discovery/targets (list/create) and
// PUT/DELETE /api/v1/discovery/targets/{id} (enable/disable, delete).
type TargetsAPI struct{ Repo TargetRepository }

type targetRequest struct {
	OrganizationID string `json:"organizationId"`
	CIDR           string `json:"cidr"`
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProto      string `json:"authProto"`
	AuthPass       string `json:"authPass"`
	PrivProto      string `json:"privProto"`
	PrivPass       string `json:"privPass"`
	Port           int    `json:"port"`
	TimeoutMS      int    `json:"timeoutMs"`
	IntervalSecs   int    `json:"intervalSeconds"`
	Enabled        *bool  `json:"enabled"`
}

func (a TargetsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		org := r.URL.Query().Get("organizationId")
		out, err := a.Repo.ListTargets(r.Context(), org)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)
	case http.MethodPost:
		var in targetRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.CIDR == "" || in.OrganizationID == "" {
			http.Error(w, "organizationId and cidr are required", http.StatusBadRequest)
			return
		}
		version := in.Version
		if version == "" {
			version = "2c"
		}
		community := in.Community
		if community == "" && version != "3" && version != "v3" {
			community = "public"
		}
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		t, err := a.Repo.CreateTarget(r.Context(), TargetInput{
			OrganizationID: in.OrganizationID,
			CIDR:           in.CIDR,
			SNMP: snmp.Credentials{
				Version:   snmp.Version(version),
				Community: community,
				Username:  in.Username,
				AuthProto: in.AuthProto,
				AuthPass:  in.AuthPass,
				PrivProto: in.PrivProto,
				PrivPass:  in.PrivPass,
			},
			SNMPPort:     uint16(in.Port),
			TimeoutMS:    in.TimeoutMS,
			IntervalSecs: in.IntervalSecs,
			Enabled:      enabled,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, t)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// TargetAPI backs PUT/DELETE /api/v1/discovery/targets/{id}. PUT here only
// toggles enabled (a full field-by-field edit isn't needed for this
// increment's scope -- disable+recreate covers changing a CIDR/interval).
type TargetAPI struct{ Repo TargetRepository }

func (a TargetAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := a.Repo.SetTargetEnabled(r.Context(), id, in.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := a.Repo.DeleteTarget(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// CandidatesAPI backs GET /api/v1/discovery/candidates -- lists persisted
// candidates for an organization (optionally filtered by targetId/status).
type CandidatesAPI struct{ Repo TargetRepository }

func (a CandidatesAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	org := r.URL.Query().Get("organizationId")
	if org == "" {
		http.Error(w, "organizationId is required", http.StatusBadRequest)
		return
	}
	var targetID int64
	if v := r.URL.Query().Get("targetId"); v != "" {
		targetID, _ = strconv.ParseInt(v, 10, 64)
	}
	status := r.URL.Query().Get("status")
	out, err := a.Repo.ListCandidates(r.Context(), org, targetID, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, out)
}

// CandidateActionAPI backs POST /api/v1/discovery/candidates/{id}/import
// and .../ignore.
type CandidateActionAPI struct {
	Repo    TargetRepository
	Devices devices.Repository
	Action  string // "import" | "ignore"
}

func (a CandidateActionAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid candidate id", http.StatusBadRequest)
		return
	}
	candidate, err := a.Repo.CandidateByID(r.Context(), id)
	if err != nil {
		http.Error(w, "candidate not found", http.StatusNotFound)
		return
	}

	if a.Action == "ignore" {
		if err := a.Repo.MarkCandidateStatus(r.Context(), id, "ignored", ""); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	target, err := a.Repo.TargetByID(r.Context(), candidate.TargetID)
	if err != nil {
		http.Error(w, "owning target not found", http.StatusInternalServerError)
		return
	}
	name := candidate.SystemName
	if name == "" {
		name = candidate.Address
	}
	record, err := a.Devices.Create(r.Context(), devices.DeviceInput{
		OrganizationID: target.OrganizationID,
		Name:           name,
		Address:        candidate.Address,
		DeviceType:     candidate.DeviceType,
		Vendor:         candidate.Vendor,
		SNMP:           target.SNMP,
		SNMPPort:       target.SNMPPort,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := a.Repo.MarkCandidateStatus(r.Context(), id, "imported", record.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, record)
}
