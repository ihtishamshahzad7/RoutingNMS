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
	Live     Result        `json:"live"`
	History  []ProbeResult `json:"history"`
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
