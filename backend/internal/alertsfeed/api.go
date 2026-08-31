package alertsfeed

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// API backs GET /api/v1/alerts/active -- the single endpoint the browser
// voice-alert feature polls.
type API struct{ Repo Repository }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	active, err := a.Repo.List(ctx)
	if err != nil {
		http.Error(w, "failed to load active alerts", http.StatusInternalServerError)
		return
	}
	if active == nil {
		active = []Active{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(active)
}
