package accesspoints

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

// API backs GET/POST /api/v1/access-points and GET/PUT/DELETE
// /api/v1/access-points/{id} -- session-authed CRUD for wireless access
// points (migration 0018).
type API struct{ Repo Repository }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		items, err := a.Repo.List(ctx, r.URL.Query().Get("tenantId"), r.URL.Query().Get("siteId"))
		if err != nil {
			http.Error(w, "failed to load access points", http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)

	case r.Method == http.MethodPost && idStr == "":
		var in Input
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		a, err := a.Repo.Create(ctx, in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, a)

	case r.Method == http.MethodGet && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid access point id", http.StatusBadRequest)
			return
		}
		ap, err := a.Repo.Get(ctx, id)
		if err != nil {
			http.Error(w, "access point not found", http.StatusNotFound)
			return
		}
		writeJSON(w, ap)

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid access point id", http.StatusBadRequest)
			return
		}
		var in Input
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		ap, err := a.Repo.Update(ctx, id, in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, ap)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid access point id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete access point", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
