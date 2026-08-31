package discovery

import (
	"encoding/json"
	"net/http"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// ScanAPI backs POST /api/v1/discovery/scan (start a scan) and
// GET /api/v1/discovery/scan/{id} (poll progress/results).
type ScanAPI struct{ Manager *Manager }

type startRequest struct {
	CIDR        string `json:"cidr"`
	Version     string `json:"version"`
	Community   string `json:"community"`
	Username    string `json:"username"`
	AuthProto   string `json:"authProto"`
	AuthPass    string `json:"authPass"`
	PrivProto   string `json:"privProto"`
	PrivPass    string `json:"privPass"`
	Port        int    `json:"port"`
	TimeoutMS   int    `json:"timeoutMs"`
	Concurrency int    `json:"concurrency"`
}

func (a ScanAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in startRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.CIDR == "" {
		http.Error(w, "cidr is required", http.StatusBadRequest)
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
	port := uint16(161)
	if in.Port > 0 && in.Port <= 65535 {
		port = uint16(in.Port)
	}
	creds := snmp.Credentials{
		Version:   snmp.Version(version),
		Community: community,
		Username:  in.Username,
		AuthProto: in.AuthProto,
		AuthPass:  in.AuthPass,
		PrivProto: in.PrivProto,
		PrivPass:  in.PrivPass,
	}

	job, err := a.Manager.Start(in.CIDR, creds, port, in.TimeoutMS, in.Concurrency)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, job)
}

// JobAPI backs GET /api/v1/discovery/scan/{id}.
type JobAPI struct{ Manager *Manager }

func (a JobAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	job, ok := a.Manager.Get(id)
	if !ok {
		http.Error(w, "scan job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, job)
}

// ImportAPI backs POST /api/v1/discovery/import: turns selected scan
// results into real monitored devices, reusing the SNMP credentials the
// scan itself was run with (so the operator doesn't have to retype a
// community string per device -- "select and add discovered devices in one
// click", matching the reference tool this feature is modeled on).
type ImportAPI struct {
	Manager *Manager
	Devices devices.Repository
}

type importRequest struct {
	JobID          string   `json:"jobId"`
	OrganizationID string   `json:"organizationId"`
	Addresses      []string `json:"addresses"`
}

type importResult struct {
	Created []devices.Record `json:"created"`
	Failed  []string         `json:"failed,omitempty"`
}

func (a ImportAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in importRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.JobID == "" || in.OrganizationID == "" || len(in.Addresses) == 0 {
		http.Error(w, "jobId, organizationId and at least one address are required", http.StatusBadRequest)
		return
	}
	job, ok := a.Manager.Get(in.JobID)
	if !ok {
		http.Error(w, "scan job not found", http.StatusNotFound)
		return
	}
	creds, port, _ := a.Manager.Credentials(in.JobID)

	wanted := map[string]bool{}
	for _, addr := range in.Addresses {
		wanted[addr] = true
	}

	out := importResult{}
	ctx := r.Context()
	for _, found := range job.Results {
		if !wanted[found.Address] {
			continue
		}
		name := found.SystemName
		if name == "" {
			name = found.Address
		}
		record, err := a.Devices.Create(ctx, devices.DeviceInput{
			OrganizationID: in.OrganizationID,
			Name:           name,
			Address:        found.Address,
			DeviceType:     found.DeviceType,
			Vendor:         found.Vendor,
			SNMP:           creds,
			SNMPPort:       port,
		})
		if err != nil {
			out.Failed = append(out.Failed, found.Address)
			continue
		}
		out.Created = append(out.Created, record)
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
