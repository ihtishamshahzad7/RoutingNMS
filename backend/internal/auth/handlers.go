package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
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
	// apiKeys, when set via WithAPIKey, lets Middleware/OptionalMiddleware
	// additionally accept a RoutingNMS API key (Authorization: Bearer
	// <key>) as an alternative to the session cookie. Nil by default, so
	// existing routes are unaffected until they opt in with WithAPIKey.
	apiKeys APIKeyVerifier
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	Username           string `json:"username"`
	MustChangePassword bool   `json:"mustChangePassword"`
	TwoFAEnabled       bool   `json:"twoFAEnabled"`
}

type twoFARequiredResponse struct {
	TwoFARequired bool   `json:"twoFARequired"`
	PendingToken  string `json:"pendingToken"`
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

	// Password checked out. If the account has 2FA enabled, do not issue a
	// session yet -- require a second request (LoginTwoFA) that proves
	// possession of the authenticator app.
	if user.TwoFAStatus {
		pendingToken, err := h.Store.CreatePendingTwoFA(r.Context(), user.ID)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not start 2fa challenge"})
			return
		}
		writeJSON(w, http.StatusOK, twoFARequiredResponse{TwoFARequired: true, PendingToken: pendingToken})
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
	writeJSON(w, http.StatusOK, userResponse{Username: user.Username, MustChangePassword: user.MustChangePassword, TwoFAEnabled: user.TwoFAStatus})
}

type loginTwoFARequest struct {
	PendingToken string `json:"pendingToken"`
	Token        string `json:"token"`
}

// LoginTwoFA handles POST /api/v1/auth/login/2fa, the second step of login
// for an account with 2FA enabled: it exchanges a pendingToken (from
// Login's twoFARequired response) plus a current TOTP token for a real
// session.
func (h Handler) LoginTwoFA(w http.ResponseWriter, r *http.Request) {
	var req loginTwoFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.PendingToken == "" || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pendingToken and token are required"})
		return
	}

	user, err := h.Store.ResolvePendingTwoFA(r.Context(), req.PendingToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "2fa challenge expired or invalid, please log in again"})
		return
	}
	if !user.TwoFAStatus {
		// 2FA was disabled between Login and this request; the pending
		// token is now meaningless.
		_ = h.Store.ConsumePendingTwoFA(r.Context(), req.PendingToken)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "2fa challenge expired or invalid, please log in again"})
		return
	}

	// Replay prevention: reject a token equal to the last one successfully
	// used for this account, even if it is otherwise still within its
	// validity window.
	valid := VerifyTOTP(user.TwoFASecret, req.Token, time.Now()) && strings.TrimSpace(req.Token) != user.TwoFALastToken
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid two-factor code"})
		return
	}

	// One-shot: whether this succeeds or fails downstream, the pending
	// token must not be reusable.
	_ = h.Store.ConsumePendingTwoFA(r.Context(), req.PendingToken)
	_ = h.Store.SetTwoFALastToken(r.Context(), user.ID, strings.TrimSpace(req.Token))

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
	writeJSON(w, http.StatusOK, userResponse{Username: user.Username, MustChangePassword: user.MustChangePassword, TwoFAEnabled: user.TwoFAStatus})
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
	writeJSON(w, http.StatusOK, userResponse{Username: user.Username, MustChangePassword: user.MustChangePassword, TwoFAEnabled: user.TwoFAStatus})
}

// twoFAIssuer is the "issuer" field embedded in the otpauth:// URI, shown
// by authenticator apps to identify which account/service a code is for.
const twoFAIssuer = "RoutingNMS"

type twoFAPasswordRequest struct {
	Password string `json:"password"`
}

type prepareTwoFAResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// requireCurrentPassword re-verifies the authenticated user's current
// password, as Kuma requires before any 2FA state change. It writes an
// error response and returns false on failure.
func (h Handler) requireCurrentPassword(w http.ResponseWriter, r *http.Request, user User, password string) bool {
	if password == "" || !VerifyPassword(user.PasswordHash, password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is required and must be correct"})
		return false
	}
	return true
}

// PrepareTwoFA handles POST /api/v1/auth/2fa/prepare: re-verifies the
// current password, generates a new TOTP secret and stores it on the user
// row without enabling 2FA yet, and returns the secret plus an otpauth://
// URI for QR-code display.
func (h Handler) PrepareTwoFA(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var req twoFAPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if !h.requireCurrentPassword(w, r, user, req.Password) {
		return
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not generate 2fa secret"})
		return
	}
	if err := h.Store.PrepareTwoFA(r.Context(), user.ID, secret); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not save 2fa secret"})
		return
	}
	writeJSON(w, http.StatusOK, prepareTwoFAResponse{
		Secret: secret,
		URI:    TOTPURI(twoFAIssuer, user.Username, secret),
	})
}

type saveTwoFARequest struct {
	Password string `json:"password"`
	Token    string `json:"token"`
}

// SaveTwoFA handles POST /api/v1/auth/2fa/save: re-verifies the current
// password AND a valid current TOTP token (proving the user's
// authenticator app is correctly configured against the secret from
// PrepareTwoFA) before enabling 2FA on the account.
func (h Handler) SaveTwoFA(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var req saveTwoFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if !h.requireCurrentPassword(w, r, user, req.Password) {
		return
	}
	if user.TwoFASecret == "" || !VerifyTOTP(user.TwoFASecret, req.Token, time.Now()) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid two-factor code"})
		return
	}
	if err := h.Store.EnableTwoFA(r.Context(), user.ID); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not enable 2fa"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DisableTwoFA handles POST /api/v1/auth/2fa/disable: re-verifies the
// current password, then disables 2FA on the account.
func (h Handler) DisableTwoFA(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var req twoFAPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if !h.requireCurrentPassword(w, r, user, req.Password) {
		return
	}
	if err := h.Store.DisableTwoFA(r.Context(), user.ID); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "could not disable 2fa"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Middleware authenticates the request's session cookie and, when valid,
// attaches the resolved User to the request context before calling next. If
// the cookie is missing or invalid it responds 401 and does not call next.
func (h Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try an API key first (Kuma's own fallback order), then the
		// session cookie -- so a request carrying a valid Authorization
		// header never needs a session at all.
		if user, ok := h.apiKeyUser(r); ok {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
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
		if user, ok := h.apiKeyUser(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
			next.ServeHTTP(w, r)
			return
		}
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

// APIKeyVerifier resolves a presented API key string to its owning user
// id. Implemented by apikeys.Repository; declared here (rather than
// importing that package, which would create an import cycle since
// apikeys imports auth for password hashing) so Middleware can accept one.
type APIKeyVerifier interface {
	Verify(ctx context.Context, key string) (userID int64, err error)
}

// WithAPIKey returns a copy of h whose Middleware additionally accepts a
// RoutingNMS API key via "Authorization: Bearer <key>", falling back to
// the session cookie when no such header is present -- the same
// try-API-key-first-else-session shape Kuma uses. Requests authenticated
// by an API key act as that key's owning user for API purposes, same as a
// normal session.
func (h Handler) WithAPIKey(verifier APIKeyVerifier) Handler {
	h.apiKeys = verifier
	return h
}

// apiKeyUser resolves the bearer token in an Authorization header (if any)
// to a User via h.apiKeys. ok is false whenever no usable API key was
// presented, which is not itself an error -- the caller falls back to
// session auth.
func (h Handler) apiKeyUser(r *http.Request) (User, bool) {
	if h.apiKeys == nil {
		return User{}, false
	}
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return User{}, false
	}
	key := strings.TrimSpace(authz[len(prefix):])
	if key == "" {
		return User{}, false
	}
	userID, err := h.apiKeys.Verify(r.Context(), key)
	if err != nil {
		return User{}, false
	}
	user, err := h.Store.UserByID(r.Context(), userID)
	if err != nil {
		return User{}, false
	}
	return user, true
}
