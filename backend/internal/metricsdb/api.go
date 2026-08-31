package metricsdb

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// API backs GET /api/v1/metrics?subjectType=device&subjectId=X&metric=latency_ms&metric=up&since=1h
type API struct{ Repo Repository }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	subjectType := strings.TrimSpace(q.Get("subjectType"))
	subjectID := strings.TrimSpace(q.Get("subjectId"))
	metrics := q["metric"]
	if subjectType == "" || subjectID == "" || len(metrics) == 0 {
		http.Error(w, "subjectType, subjectId and at least one metric are required", http.StatusBadRequest)
		return
	}
	since := 24 * time.Hour
	if raw := q.Get("since"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			since = d
		}
	}
	if since > 30*24*time.Hour {
		since = 30 * 24 * time.Hour
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	series, err := a.Repo.Query(ctx, subjectType, subjectID, metrics, since)
	if err != nil {
		http.Error(w, "failed to load metric history", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(series)
}
