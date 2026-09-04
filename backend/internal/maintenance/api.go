package maintenance

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

// AdminAPI backs GET/POST /api/v1/maintenance-windows and
// GET/PUT/DELETE /api/v1/maintenance-windows/{id} -- session-authed CRUD.
type AdminAPI struct{ Repo Repository }

func (a AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		windows, err := a.Repo.List(ctx, r.URL.Query().Get("tenantId"))
		if err != nil {
			http.Error(w, "failed to load maintenance windows", http.StatusInternalServerError)
			return
		}
		writeJSON(w, windows)

	case r.Method == http.MethodPost && idStr == "":
		var win Window
		if err := json.NewDecoder(r.Body).Decode(&win); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		created, err := a.Repo.Create(ctx, win)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, created)

	case r.Method == http.MethodGet && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid maintenance window id", http.StatusBadRequest)
			return
		}
		win, err := a.Repo.Get(ctx, id)
		if err != nil {
			http.Error(w, "maintenance window not found", http.StatusNotFound)
			return
		}
		items, err := a.Repo.ListItems(ctx, id)
		if err != nil {
			http.Error(w, "failed to load window items", http.StatusInternalServerError)
			return
		}
		writeJSON(w, struct {
			Window
			Items []Item `json:"items"`
		}{win, items})

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid maintenance window id", http.StatusBadRequest)
			return
		}
		var win Window
		if err := json.NewDecoder(r.Body).Decode(&win); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.Update(ctx, id, win)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, updated)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid maintenance window id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete maintenance window", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ItemsAPI backs PUT /api/v1/maintenance-windows/{id}/items -- session-authed,
// replaces the full set of devices/OLTs a window applies to.
type ItemsAPI struct{ Repo Repository }

func (a ItemsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid maintenance window id", http.StatusBadRequest)
		return
	}
	var req struct {
		Items []Item `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := a.Repo.ReplaceItems(ctx, id, req.Items); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items, err := a.Repo.ListItems(ctx, id)
	if err != nil {
		http.Error(w, "failed to reload items", http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
}
