package topology

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// GraphHandler serves the full persisted topology graph (inventory nodes plus
// active links from topology_links) at GET /api/topology. It replaces the
// earlier empty-links graph now that the discovery loop persists links.
type GraphHandler struct{ Repo Repository }

func (h GraphHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Repo.DB == nil {
		http.Error(w, "topology provider is not initialized", http.StatusServiceUnavailable)
		return
	}
	g, err := h.Repo.Graph(r.Context())
	if err != nil {
		http.Error(w, "failed to load topology: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(g)
}

// SnapshotHandler lists recent topology snapshots (time-travel) at
// GET /api/v1/topology/snapshots.
type SnapshotHandler struct{ Repo Repository }

func (h SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 24
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	snapshots, err := h.Repo.ListSnapshots(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to load snapshots: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshots)
}

// DiscoverHandler triggers a manual discovery cycle at
// POST /api/v1/topology/discover.
type DiscoverHandler struct{ Engine *Discovery }

func (h DiscoverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Engine == nil {
		http.Error(w, "topology discovery is not initialized", http.StatusServiceUnavailable)
		return
	}
	links, err := h.Engine.DiscoverNow(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "links": links})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "links": links, "status": h.Engine.Status()})
}

// StatusHandler reports the last discovery cycle outcome at
// GET /api/v1/topology/status.
type StatusHandler struct{ Engine *Discovery }

func (h StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Engine == nil {
		http.Error(w, "topology discovery is not initialized", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.Engine.Status())
}
