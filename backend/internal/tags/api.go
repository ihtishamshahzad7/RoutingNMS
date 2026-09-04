package tags

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

// AdminAPI backs GET/POST /api/v1/tags and GET(unused)/PUT/DELETE
// /api/v1/tags/{id} -- session-authed CRUD for tag definitions themselves.
type AdminAPI struct{ Repo Repository }

func (a AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		list, err := a.Repo.List(ctx, r.URL.Query().Get("tenantId"))
		if err != nil {
			http.Error(w, "failed to load tags", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)

	case r.Method == http.MethodPost && idStr == "":
		var t Tag
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		created, err := a.Repo.Create(ctx, t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, created)

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid tag id", http.StatusBadRequest)
			return
		}
		var t Tag
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.Update(ctx, id, t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, updated)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid tag id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete tag", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// AssignmentsAPI backs:
//
//	GET  /api/v1/tags/assignments                       -- every assignment (for annotating list views)
//	GET  /api/v1/tag-assignments/{subjectType}/{subjectId} -- tags on one subject
//	PUT  /api/v1/tag-assignments/{subjectType}/{subjectId} -- replace the full tag list for one subject
type AssignmentsAPI struct{ Repo Repository }

func (a AssignmentsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	subjectType := r.PathValue("subjectType")
	subjectID := r.PathValue("subjectId")

	switch {
	case r.Method == http.MethodGet && subjectType == "" && subjectID == "":
		all, err := a.Repo.AllAssignments(ctx)
		if err != nil {
			http.Error(w, "failed to load tag assignments", http.StatusInternalServerError)
			return
		}
		writeJSON(w, all)

	case r.Method == http.MethodGet:
		list, err := a.Repo.ForSubject(ctx, subjectType, subjectID)
		if err != nil {
			http.Error(w, "failed to load tags for subject", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)

	case r.Method == http.MethodPut:
		var req struct {
			TagIDs []int64 `json:"tagIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := a.Repo.ReplaceForSubject(ctx, subjectType, subjectID, req.TagIDs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		list, err := a.Repo.ForSubject(ctx, subjectType, subjectID)
		if err != nil {
			http.Error(w, "failed to reload tags", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
