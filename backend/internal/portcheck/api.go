package portcheck

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// API exposes Port/Service-check live status and on-demand checks for the
// frontend, mirroring sshcheck.API's shape.
type API struct {
	Devices devices.Repository
	Poller  *Poller
	Metrics metricsdb.Repository
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

// HistoryRangePoint is one Port/Service check sample -- shaped like ping's
// ProbeResult (probedAt/latencyMs/reachable) so the device-detail chart
// (item 2.2) can reuse the same rendering code as the ICMP graph.
type HistoryRangePoint struct {
	ProbedAt  time.Time `json:"probedAt"`
	LatencyMS *float64  `json:"latencyMs,omitempty"`
	Reachable bool      `json:"isReachable"`
}

// HistoryRange serves GET /api/v1/port-check/{id}/history-range?range=
// 1h|24h|7d -- reads the port_check_up/port_check_latency_ms samples the
// Poller already writes to metricsdb (RecordBatch in poller.go) over the
// requested window. The two metrics are recorded together, once per poll,
// so they line up index-for-index; this zips them back into one series.
func (a API) HistoryRange(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r.URL.Path, "history-range")
	if !ok {
		http.NotFound(w, r)
		return
	}
	var window time.Duration
	switch r.URL.Query().Get("range") {
	case "1h":
		window = time.Hour
	case "7d":
		window = 7 * 24 * time.Hour
	default:
		window = 24 * time.Hour
	}
	series, err := a.Metrics.Query(r.Context(), "device", id, []string{"port_check_up", "port_check_latency_ms"}, window)
	if err != nil {
		http.Error(w, "failed to load port-check history", 500)
		return
	}
	var upSeries, latSeries metricsdb.Series
	for _, s := range series {
		switch s.Metric {
		case "port_check_up":
			upSeries = s
		case "port_check_latency_ms":
			latSeries = s
		}
	}
	out := make([]HistoryRangePoint, 0, len(upSeries.Points))
	for i, p := range upSeries.Points {
		point := HistoryRangePoint{ProbedAt: p.Timestamp, Reachable: p.Value != 0}
		if i < len(latSeries.Points) {
			lat := latSeries.Points[i].Value
			point.LatencyMS = &lat
		}
		out = append(out, point)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"history": out})
}

func pathID(path, suffix string) (string, bool) {
	path = strings.TrimPrefix(path, "/api/v1/port-check/")
	path = strings.TrimSuffix(path, "/"+suffix)
	if path == "" {
		return "", false
	}
	return path, true
}
