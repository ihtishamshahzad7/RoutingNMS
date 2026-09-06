package tenants

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RetentionAPI backs GET/PUT /api/v1/tenants/{id}/retention, the Settings
// page control for how many days of monitoring history (metric_samples) a
// tenant keeps -- RoutingNMS's per-tenant equivalent of Uptime Kuma's global
// "Keep Data Period" setting.
type RetentionAPI struct{ Repo Repository }

type retentionResponse struct {
	TenantID          string `json:"tenantId"`
	DataRetentionDays int    `json:"dataRetentionDays"`
}

func (a RetentionAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.PathValue("id"))
	if tenantID == "" {
		http.Error(w, "tenant id is required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		days, err := a.Repo.RetentionDays(r.Context(), tenantID)
		if err != nil {
			http.Error(w, "failed to load retention setting", http.StatusInternalServerError)
			return
		}
		writeJSON(w, retentionResponse{TenantID: tenantID, DataRetentionDays: days})
	case http.MethodPut:
		var req retentionResponse
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// No lower bound enforced here: matching Kuma, any value < 1
		// (including 0 or negative) means "keep forever" -- the retention
		// job (internal/retention) is what actually interprets that, this
		// endpoint just stores whatever the operator asked for.
		if err := a.Repo.SetRetentionDays(r.Context(), tenantID, req.DataRetentionDays); err != nil {
			http.Error(w, "failed to save retention setting", http.StatusInternalServerError)
			return
		}
		writeJSON(w, retentionResponse{TenantID: tenantID, DataRetentionDays: req.DataRetentionDays})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
