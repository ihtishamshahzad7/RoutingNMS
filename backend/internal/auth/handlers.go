package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// CookieName is the name of the session cookie set on successful login.
const CookieName = "routingnms_session"

type contextKey string

const userContextKey contextKey = "routingnms.user"

// Handler wires the auth HTTP endpoints against a Store.
type Handler struct {
	Store Store
	// Secure controls the cookie's Secure flag. It should be true whenever
	// RoutingNMS is served over HTTPS. It defaults to false so a fresh
	// HTTP-only installation (the documented deployment) is not locked out
	// of its own login page.
	Secure bool
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	Username           string `json:"username"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Login handles POST /api/v1/auth/login.
func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	user, err := h.Store.UserByUsername(r.Context(), username)
	if err != nil && err != ErrInvalidCredentials {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authentication service unavailable"})
		return
	}
	// Always run VerifyPassword, even on a lookup miss, against a fixed
	// dummy hash so the endpoint's timing does not reveal whether a
	// username exists.
	hash := user.PasswordHash
	if err == ErrInvalidCredentials {
		hash = "pbkdf2-sha256$210000$00000000000000000000000000000000$0000000000000000000000000000000000000000000000000000000000000000"
	}
	if !VerifyPassword(hash, req.Password) || err == ErrInvalidCredentials {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	token, expiresAt, err := h.Store.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not create session"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, userResponse{Username: user.Username, MustChangePassword: user.MustChangePassword})
}

// Logout handles POST /api/v1/auth/logout.
func (h Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		_ = h.Store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Me handles GET /api/v1/auth/me, returning the current session's user or
// 401 if there is none.
func (h Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, userResponse{Username: user.Username, MustChangePassword: user.MustChangePassword})
}

// Middleware authenticates the request's session cookie and, when valid,
// attaches the resolved User to the request context before calling next. If
// the cookie is missing or invalid it responds 401 and does not call next.
func (h Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil || cookie.Value == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		user, err := h.Store.SessionUser(r.Context(), cookie.Value)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired or invalid"})
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalMiddleware behaves like Middleware but never rejects the request:
// it simply attaches the user to the context when a valid session cookie is
// present. Used for endpoints such as /api/v1/auth/me that need to report
// "not authenticated" as a normal JSON response rather than failing the
// whole handler chain.
func (h Handler) OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(CookieName); err == nil && cookie.Value != "" {
			if user, err := h.Store.SessionUser(r.Context(), cookie.Value); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext returns the authenticated User attached by Middleware or
// OptionalMiddleware, if any.
func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey).(User)
	return user, ok
}
