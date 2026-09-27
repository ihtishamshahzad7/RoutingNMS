package portcheck

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
)

// API exposes Port/Service-check live status and on-demand checks for the
// frontend, mirroring sshcheck.API's shape.
type API struct {
	Devices devices.Repository
	Poller  *Poller
}

// Live serves GET /api/v1/port-check/{id}/live.
func (a API) Live(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r.URL.Path, "live")
	if !ok {
		http.NotFound(w, r)
		return
	}
	var live Result
	if a.Poller != nil {
		live, _ = a.Poller.Live(id)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"live": live})
}

// Check serves POST /api/v1/port-check/{id}/check -- forces an immediate check.
func (a API) Check(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r.URL.Path, "check")
	if !ok {
		http.NotFound(w, r)
		return
	}
	dev, err := a.Devices.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "device not found", 404)
		return
	}
	var codes []string
	for _, c := range strings.Split(dev.PortCheckAcceptedStatusCodes, ",") {
		if c = strings.TrimSpace(c); c != "" {
			codes = append(codes, c)
		}
	}
	d := EnabledDevice{ID: dev.ID, Address: dev.Address, Protocol: dev.PortCheckProtocol, Port: dev.PortCheckPort, Path: dev.PortCheckPath, AcceptedCodes: codes}
	var res Result
	if a.Poller != nil {
		res = a.Poller.Force(r.Context(), d)
	} else {
		res = Check(r.Context(), d.Protocol, d.Address, d.Port, d.Path, d.AcceptedCodes, 5*time.Second)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// Summary serves GET /api/v1/port-check/summary?organizationId=... -- the
// fleet-wide "which devices have Port/Service check configured, and what's
// their live status" list the Connectivity Monitoring page (item 2.2) shows
// side by side with each device's ICMP status.
func (a API) Summary(w http.ResponseWriter, r *http.Request) {
	org := r.URL.Query().Get("organizationId")
	if org == "" {
		http.Error(w, "organizationId is required", http.StatusBadRequest)
		return
	}
	list, err := a.Devices.List(r.Context(), org)
	if err != nil {
		http.Error(w, "failed to load devices", 500)
		return
	}
	live := map[string]Result{}
	if a.Poller != nil {
		live = a.Poller.LiveAll()
	}
	type row struct {
		DeviceID  string `json:"deviceId"`
		Protocol  string `json:"protocol"`
		Port      int    `json:"port"`
		Reachable bool   `json:"reachable"`
		LatencyMS float64 `json:"latencyMs"`
	}
	out := []row{}
	for _, d := range list {
		if !d.PortCheckEnabled {
			continue
		}
		res := live[d.ID]
		out = append(out, row{DeviceID: d.ID, Protocol: d.PortCheckProtocol, Port: d.PortCheckPort, Reachable: res.Reachable, LatencyMS: res.LatencyMS})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"devices": out})
}

func pathID(path, suffix string) (string, bool) {
	path = strings.TrimPrefix(path, "/api/v1/port-check/")
	path = strings.TrimSuffix(path, "/"+suffix)
	if path == "" {
		return "", false
	}
	return path, true
}
