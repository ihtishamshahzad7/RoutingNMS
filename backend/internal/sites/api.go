package sites

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// API backs GET/POST /api/v1/sites and GET/PUT/DELETE /api/v1/sites/{id} --
// session-authed CRUD for physical site locations (migration 0018).
type API struct{ Repo Repository }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	tenantID := r.URL.Query().Get("tenantId")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		items, err := a.Repo.List(ctx, tenantID)
		if err != nil {
			http.Error(w, "failed to load sites", http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)

	case r.Method == http.MethodPost && idStr == "":
		var in SiteInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		s, err := a.Repo.Create(ctx, in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, s)

	case r.Method == http.MethodGet && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid site id", http.StatusBadRequest)
			return
		}
		s, err := a.Repo.Get(ctx, id)
		if err != nil {
			http.Error(w, "site not found", http.StatusNotFound)
			return
		}
		writeJSON(w, s)

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid site id", http.StatusBadRequest)
			return
		}
		var in SiteInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		s, err := a.Repo.Update(ctx, id, in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, s)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid site id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete site", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
