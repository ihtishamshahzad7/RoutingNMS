package workspacetopology

// Real-time push for the Workspace Topology Builder's monitoring panel.
// Closes part of the "replace the 5-second mock metrics tick with real
// device polling" follow-up flagged since feature 32/33/34 -- specifically
// the up/down state and latency, since those are the only per-device
// metrics RoutingNMS actually collects and persists into metric_samples
// today (see devices/sampler.go's "up"/"latency_ms" samples and
// ping/poller.go's "icmp_rtt_ms"). Bandwidth/CPU/memory remain the
// existing client-side mock (mockMetrics.ts) since no collector for those
// exists anywhere in this codebase yet -- deliberately not silently
// faked as "real" here, and still flagged as an open follow-up.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Reading is one linked canvas device's real, current up/latency state,
// as pushed to subscribed clients over SSE.
type Reading struct {
	DeviceID       string   `json:"deviceId"`       // workspace_topology_devices.id (canvas device)
	LinkedDeviceID string   `json:"linkedDeviceId"` // the real devices.id it's linked to
	Up             *bool    `json:"up,omitempty"`
	LatencyMs      *float64 `json:"latencyMs,omitempty"`
}

// LiveReadings resolves the current up/latency state for every device in
// groupID that has a linkedDeviceId, by reading the same metric_samples
// rows the ICMP poller and device sampler already write -- this package
// does no polling of its own, it only presents data other packages own
// (the same read-only-presentation convention statuspage.StatusResolver
// already established).
func (r Repository) LiveReadings(ctx context.Context, groupID int64) ([]Reading, error) {
	if r.DB == nil {
		return nil, nil
	}
	devices, err := r.DevicesOf(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]Reading, 0, len(devices))
	for _, d := range devices {
		if d.LinkedDeviceID == nil || *d.LinkedDeviceID == "" {
			continue
		}
		rd := Reading{DeviceID: d.ID, LinkedDeviceID: *d.LinkedDeviceID}

		var upVal float64
		if err := r.DB.QueryRow(ctx,
			`SELECT value FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='up' ORDER BY recorded_at DESC LIMIT 1`,
			*d.LinkedDeviceID).Scan(&upVal); err == nil {
			up := upVal == 1
			rd.Up = &up
		}

		var latency float64
		// Prefer the ICMP poller's own "icmp_rtt_ms" (higher-resolution,
		// dedicated ping cadence); fall back to the device sampler's
		// coarser "latency_ms" if a device has no ICMP samples yet (e.g.
		// ICMP disabled but the periodic health check still ran).
		if err := r.DB.QueryRow(ctx,
			`SELECT value FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='icmp_rtt_ms' ORDER BY recorded_at DESC LIMIT 1`,
			*d.LinkedDeviceID).Scan(&latency); err == nil {
			rd.LatencyMs = &latency
		} else if err := r.DB.QueryRow(ctx,
			`SELECT value FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='latency_ms' ORDER BY recorded_at DESC LIMIT 1`,
			*d.LinkedDeviceID).Scan(&latency); err == nil {
			rd.LatencyMs = &latency
		}

		out = append(out, rd)
	}
	return out, nil
}

// LiveHub is a per-group SSE fanout, modeled directly on
// internal/incidents.Stream (this codebase's existing SSE pattern --
// "Realtime is SSE, no WebSocket" per AGENTS.md) but scoped per group:
// a client only receives readings for the one group it opened the stream
// for, and the background poll loop only queries groups that currently
// have at least one subscriber, so an idle group costs nothing.
type LiveHub struct {
	Repo Repository

	mu       sync.RWMutex
	clients  map[int64]map[chan []Reading]struct{}
	interval time.Duration
}

func NewLiveHub(repo Repository) *LiveHub {
	return &LiveHub{Repo: repo, clients: map[int64]map[chan []Reading]struct{}{}, interval: 5 * time.Second}
}

func (h *LiveHub) subscribe(groupID int64) chan []Reading {
	ch := make(chan []Reading, 4)
	h.mu.Lock()
	if h.clients[groupID] == nil {
		h.clients[groupID] = map[chan []Reading]struct{}{}
	}
	h.clients[groupID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *LiveHub) unsubscribe(groupID int64, ch chan []Reading) {
	h.mu.Lock()
	delete(h.clients[groupID], ch)
	if len(h.clients[groupID]) == 0 {
		delete(h.clients, groupID)
	}
	close(ch)
	h.mu.Unlock()
}

// activeGroups returns the ids of every group with at least one live
// subscriber right now -- the poll loop only queries these.
func (h *LiveHub) activeGroups() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]int64, 0, len(h.clients))
	for gid := range h.clients {
		out = append(out, gid)
	}
	return out
}

func (h *LiveHub) publish(groupID int64, readings []Reading) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients[groupID] {
		select {
		case ch <- readings:
		default:
		}
	}
}

// Run polls every subscribed-to group on Interval and pushes fresh
// readings to its subscribers. Safe to run as a single long-lived
// goroutine for the process lifetime (mirrors alertEvaluator.Run /
// topologyEngine.Run's existing `go x.Run(ctx)` convention in main.go).
func (h *LiveHub) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, gid := range h.activeGroups() {
				readings, err := h.Repo.LiveReadings(ctx, gid)
				if err != nil {
					continue
				}
				h.publish(gid, readings)
			}
		}
	}
}

// ServeHTTP backs GET /api/v1/workspace-topology/groups/{id}/stream.
func (h *LiveHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.subscribe(groupID)
	defer h.unsubscribe(groupID, ch)

	// Send an initial snapshot immediately so the panel doesn't sit blank
	// for up to a full interval after opening.
	if readings, err := h.Repo.LiveReadings(r.Context(), groupID); err == nil {
		writeReadingsEvent(w, readings)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case readings := <-ch:
			writeReadingsEvent(w, readings)
			flusher.Flush()
		}
	}
}

func writeReadingsEvent(w http.ResponseWriter, readings []Reading) {
	b, _ := json.Marshal(readings)
	_, _ = w.Write([]byte("event: readings\ndata: " + string(b) + "\n\n"))
}
