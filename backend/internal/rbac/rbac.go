// Package rbac is Phase 0.1 of the RoutingNMS build blueprint: roles and
// permissions layered onto the existing session-cookie auth
// (internal/auth) and the existing `tenants` table (migration 0019,
// internal/tenants), rather than a JWT/UUID rewrite of either -- this
// codebase's own convention (TEXT tenant ids, "" = unattributed, session
// cookies not JWT) is kept, matching the project's standing "map onto the
// existing system, don't rewrite what already works" rule.
//
// Scope, stated plainly rather than silently narrowed: this feature adds
// the schema (migration 0044), a Repository to query it, and a
// RequirePermission middleware -- and wires that middleware onto exactly
// one new route (RolesAPI) as a working proof. It does NOT retrofit every
// existing device/alert/OLT/topology endpoint with permission checks in
// this pass. Doing that safely means auditing each route's current
// behavior one at a time (some are called by the frontend with no user
// context at all yet, e.g. public status-page/badge routes must stay
// unauthenticated) -- exactly the kind of change the project's own
// one-feature-at-a-time discipline exists to avoid rushing. Retrofitting
// enforcement onto existing routes is the explicit next increment (0.1b),
// not bundled here.
package rbac

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/auth"
)

type Role struct {
	ID           int64  `json:"id"`
	TenantID     string `json:"tenantId"`
	Name         string `json:"name"`
	IsSystemRole bool   `json:"isSystemRole"`
}

type Repository struct{ DB *pgxpool.Pool }

// Permissions returns the union of permission keys granted to userID across
// every role assignment it holds, in every tenant. There is no per-request
// "active tenant" in the current session model (the cookie identifies a
// user, not a user+tenant pair), so this is deliberately unscoped rather
// than guessing a tenant -- the same tradeoff this codebase already made
// for several list endpoints (see claude/roadmap.md's "leave the read side
// unscoped" pattern from features 27/28). A future increment that adds an
// active-tenant selector to the session can narrow this by tenant_id.
func (r Repository) Permissions(ctx context.Context, userID int64) ([]string, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("rbac repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT DISTINCT p.key
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1
		ORDER BY p.key`, userID)
	if err != nil {
		return nil, fmt.Errorf("query permissions: %w", err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// RoleNames returns the distinct role names userID holds, across tenants.
func (r Repository) RoleNames(ctx context.Context, userID int64) ([]string, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("rbac repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT DISTINCT rl.name
		FROM user_roles ur JOIN roles rl ON rl.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY rl.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("query role names: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan role name: %w", err)
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// HasPermission reports whether userID holds the given permission key
// through any role assignment. Errors are treated as "no" by callers via
// RequirePermission -- a DB hiccup must never fail open into an
// unauthorized action succeeding.
func (r Repository) HasPermission(ctx context.Context, userID int64, key string) (bool, error) {
	if r.DB == nil {
		return false, fmt.Errorf("rbac repository is not initialized")
	}
	var exists bool
	err := r.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = $1 AND p.key = $2
		)`, userID, key).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return exists, nil
}

// ListRoles returns every role visible to an admin (system-wide roles plus
// any tenant-scoped ones), for the roles-management screen.
func (r Repository) ListRoles(ctx context.Context) ([]Role, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("rbac repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id, tenant_id, name, is_system_role FROM roles ORDER BY tenant_id, name`)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.TenantID, &role.Name, &role.IsSystemRole); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		out = append(out, role)
	}
	return out, rows.Err()
}

// RequirePermission wraps next so it only runs when the authenticated user
// (attached to the request context by auth.Middleware, which must run
// first) holds the given permission key. Responds 401 if there is no
// authenticated user at all (auth.Middleware normally already rejects
// that, but this stays safe if RequirePermission is ever composed without
// it), 403 if the user lacks the permission, and 500 -- never a silent
// pass-through -- if the permission check itself errors.
func RequirePermission(repo Repository, key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "not authenticated", http.StatusUnauthorized)
				return
			}
			granted, err := repo.HasPermission(r.Context(), user.ID, key)
			if err != nil {
				http.Error(w, "permission check failed", http.StatusInternalServerError)
				return
			}
			if !granted {
				http.Error(w, "forbidden: missing permission "+key, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
