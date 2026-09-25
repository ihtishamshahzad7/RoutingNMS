package rbac

import (
	"encoding/json"
	"net/http"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/auth"
)

// RolesAPI backs GET /api/v1/roles -- gated by RequirePermission("role.manage")
// in main.go, the one route this increment actually enforces end to end
// (see the package doc comment for what's deliberately not enforced yet).
type RolesAPI struct{ Repo Repository }

func (a RolesAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	roles, err := a.Repo.ListRoles(r.Context())
	if err != nil {
		http.Error(w, "failed to load roles", http.StatusInternalServerError)
		return
	}
	writeJSON(w, roles)
}

// PermissionsAPI backs GET /api/v1/auth/permissions -- the current
// session's role names + permission keys, for the frontend's
// useHasPermission hook. Deliberately a separate endpoint from the
// existing auth.Handler.Me rather than editing that handler's response
// shape, to keep this addition's blast radius to zero on the
// already-working login flow.
type PermissionsAPI struct{ Repo Repository }

type permissionsResponse struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

func (a PermissionsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	roles, err := a.Repo.RoleNames(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to load roles", http.StatusInternalServerError)
		return
	}
	perms, err := a.Repo.Permissions(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to load permissions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, permissionsResponse{Roles: roles, Permissions: perms})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
