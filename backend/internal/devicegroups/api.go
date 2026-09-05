package devicegroups

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

// AdminAPI backs GET/POST /api/v1/device-groups and GET(unused)/PUT/DELETE
// /api/v1/device-groups/{id} -- session-authed CRUD for group definitions
// themselves.
type AdminAPI struct{ Repo Repository }

func (a AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		list, err := a.Repo.List(ctx, r.URL.Query().Get("tenantId"))
		if err != nil {
			http.Error(w, "failed to load device groups", http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)

	case r.Method == http.MethodPost && idStr == "":
		var g Group
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		created, err := a.Repo.Create(ctx, g)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, created)

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid group id", http.StatusBadRequest)
			return
		}
		var g Group
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.Update(ctx, id, g)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, updated)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid group id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete device group", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// MembersAPI backs:
//
//	GET /api/v1/device-groups/members                          -- every membership (for annotating list views)
//	PUT /api/v1/device-group-assignments/{subjectType}/{subjectId} -- assign/clear the group for one subject
//	PUT /api/v1/device-groups/{id}/reorder                      -- reorder members within one group
type MembersAPI struct{ Repo Repository }

func (a MembersAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	all, err := a.Repo.AllMembers(ctx)
	if err != nil {
		http.Error(w, "failed to load device group members", http.StatusInternalServerError)
		return
	}
	writeJSON(w, all)
}

// AssignmentAPI backs PUT /api/v1/device-group-assignments/{subjectType}/{subjectId}.
type AssignmentAPI struct{ Repo Repository }

func (a AssignmentAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	subjectType := r.PathValue("subjectType")
	subjectID := r.PathValue("subjectId")
	var req struct {
		GroupID   *int64 `json:"groupId"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := a.Repo.SetForSubject(ctx, subjectType, subjectID, req.GroupID, req.SortOrder); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderAPI backs PUT /api/v1/device-groups/{id}/reorder.
type ReorderAPI struct{ Repo Repository }

func (a ReorderAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	var req struct {
		SubjectIDs []string `json:"subjectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := a.Repo.Reorder(ctx, id, req.SubjectIDs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	members, err := a.Repo.MembersOf(ctx, id)
	if err != nil {
		http.Error(w, "failed to reload members", http.StatusInternalServerError)
		return
	}
	writeJSON(w, members)
}
