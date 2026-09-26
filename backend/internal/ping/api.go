package ping

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
)

// API exposes ICMP ping history and on-demand probing for the frontend's
// device detail page. Handlers assume the caller is already authenticated via
// authHandler.Middleware (matching every other API route).
type API struct {
	Repo      Repository
	Devices   devices.Repository
	Poller    *Poller
	ProbeFunc ProbeFunc
}

// liveResponse combines the recent history and the live status for a device.
type liveResponse struct {
	Live    Result        `json:"live"`
	History []ProbeResult `json:"history"`
}

// Live serves GET /api/v1/ping/{id}/live - returns the most recent probe
// result plus the last 60 results for a sparkline.
func (a API) Live(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	history, err := a.Repo.History(r.Context(), id, 60)
	if err != nil {
		writeErr(w, err)
		return
	}
	live, _ := a.Poller.Live(id)
	_ = json.NewEncoder(w).Encode(liveResponse{Live: live, History: history})
}

// History serves GET /api/v1/ping/{id}/history - paginated (limit) history.
func (a API) History(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	history, err := a.Repo.History(r.Context(), id, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"history": history})
}

// Probe serves POST /api/v1/ping/{id}/probe - forces an immediate ICMP probe
// and returns the result (used by a "Ping now" button).
func (a API) Probe(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	dev, err := a.Devices.GetByID(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	icmpDev := IcmpEnabledDevice{
		ID: dev.ID, Address: dev.Address,
		IntervalSeconds: 30, PacketSize: 56, Count: 3,
	}
	probe := a.ProbeFunc
	if probe == nil {
		probe = ExecPing
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res := probe(ctx, icmpDev)

	if a.Poller != nil {
		a.Poller.mu.Lock()
		a.Poller.live[id] = res
		a.Poller.mu.Unlock()
	}
	if did, err := strconv.ParseInt(id, 10, 64); err == nil {
		rtt, jit, ttl := res.RTTMs, res.JitterMs, res.TTL
		_ = a.Repo.Store(ctx, ProbeResult{
			DeviceID: did, ProbedAt: res.ProbedAt, RTTMs: &rtt, JitterMs: &jit,
			LossPct: res.LossPct, TTL: &ttl, Reachable: res.Reachable,
		})
	}
	_ = json.NewEncoder(w).Encode(res)
}

// DeviceUptime is one row of the fleet-wide uptime summary (Feature 1.3 --
// Core Dashboard). Status is derived server-side so the dashboard doesn't
// have to re-implement the up/down/warning/unknown rule.
type DeviceUptime struct {
	DeviceID  string   `json:"deviceId"`
	Name      string   `json:"name"`
	Address   string   `json:"address"`
	Status    string   `json:"status"` // "up" | "down" | "warning" | "unknown"
	Uptime24h *float64 `json:"uptime24h,omitempty"`
	Uptime7d  *float64 `json:"uptime7d,omitempty"`
}

// UptimeSummaryResponse is the payload for GET /api/v1/devices/uptime-summary.
type UptimeSummaryResponse struct {
	Devices []DeviceUptime `json:"devices"`
	Up      int            `json:"up"`
	Down    int            `json:"down"`
	Warning int            `json:"warning"`
	Unknown int            `json:"unknown"`
	Total   int            `json:"total"`
}

// UptimeSummary serves GET /api/v1/devices/uptime-summary?organizationId=...
// -- an aggregate Up/Down/Warning/Total tile plus a per-device 24h/7d uptime
// percentage, computed from the ICMP ping_results history that already backs
// the device-detail Ping tab. A device with icmp_enabled=false, or one that
// hasn't been probed yet, has no ping_results rows and is reported "unknown"
// rather than folded into "down".
func (a API) UptimeSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	org := r.URL.Query().Get("organizationId")
	if org == "" {
		http.Error(w, "organizationId is required", http.StatusBadRequest)
		return
	}
	list, err := a.Devices.List(r.Context(), org)
	if err != nil {
		writeErr(w, err)
		return
	}
	ids := make([]string, len(list))
	for i, d := range list {
		ids[i] = d.ID
	}
	summaries, err := a.Repo.UptimeSummaries(r.Context(), ids)
	if err != nil {
		writeErr(w, err)
		return
	}

	resp := UptimeSummaryResponse{Devices: make([]DeviceUptime, len(list)), Total: len(list)}
	for i, d := range list {
		s := summaries[d.ID]
		du := DeviceUptime{DeviceID: d.ID, Name: d.Name, Address: d.Address, Uptime24h: s.Uptime24h, Uptime7d: s.Uptime7d}
		switch {
		case s.LastReachable == nil:
			du.Status = "unknown"
			resp.Unknown++
		case !*s.LastReachable:
			du.Status = "down"
			resp.Down++
		case s.LastLossPct != nil && *s.LastLossPct > 0:
			du.Status = "warning"
			resp.Warning++
		default:
			du.Status = "up"
			resp.Up++
		}
		resp.Devices[i] = du
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func pathID(r *http.Request) (string, bool) {
	// Path form: /api/v1/ping/{id}/live or /api/v1/ping/{id}/history or
	// /api/v1/ping/{id}/probe. The id is the second-to-last segment.
	parts := splitPath(r.URL.Path)
	if len(parts) < 3 {
		return "", false
	}
	return parts[len(parts)-2], true
}

func splitPath(p string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(p[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func writeErr(w http.ResponseWriter, err error) {
	log.Printf("ping api: %v", err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
