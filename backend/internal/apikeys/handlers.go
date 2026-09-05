package apikeys

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/auth"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// API backs the session-authed CRUD endpoints an operator uses to manage
// their own API keys:
//
//	GET    /api/v1/auth/api-keys        list this user's keys
//	POST   /api/v1/auth/api-keys        create a key (returns the raw key once)
//	PUT    /api/v1/auth/api-keys/{id}   enable/disable (revoke) a key
//	DELETE /api/v1/auth/api-keys/{id}   permanently delete a key
type API struct{ Repo Repository }

type createRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type createResponse struct {
	Record
	Key string `json:"key"`
}

type setActiveRequest struct {
	Active bool `json:"active"`
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	idStr := r.PathValue("id")
	ctx := r.Context()

	switch {
	case r.Method == http.MethodGet && idStr == "":
		list, err := a.Repo.List(ctx, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load api keys"})
			return
		}
		writeJSON(w, http.StatusOK, list)

	case r.Method == http.MethodPost && idStr == "":
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		rec, key, err := a.Repo.Create(ctx, user.ID, req.Name, req.ExpiresAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, createResponse{Record: rec, Key: key})

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid api key id"})
			return
		}
		var req setActiveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := a.Repo.SetActive(ctx, user.ID, id, req.Active); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid api key id"})
			return
		}
		if err := a.Repo.Delete(ctx, user.ID, id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
