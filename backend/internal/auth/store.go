package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionTTL controls how long an issued session cookie/token remains valid.
const SessionTTL = 12 * time.Hour

// DefaultAdminUsername/DefaultAdminPassword are the credentials RoutingNMS
// bootstraps on a fresh installation when the users table is empty. They
// are intentionally well known: the operator is expected to change them
// after first login (the login UI and README say so explicitly).
const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin123"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

// User is a RoutingNMS operator account.
type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	MustChangePassword bool
	TwoFASecret        string
	TwoFAStatus        bool
	TwoFALastToken     string
}

// Store persists users and sessions in PostgreSQL.
type Store struct {
	DB *pgxpool.Pool
}

// Bootstrap creates the default admin account if (and only if) the users
// table is currently empty. It is safe to call on every startup: once any
// user exists it is a no-op, so it never resets an operator's changed
// password and never runs against an already-populated production database.
func (s Store) Bootstrap(ctx context.Context) error {
	var count int64
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}
	hash, err := HashPassword(DefaultAdminPassword)
	if err != nil {
		return fmt.Errorf("hash default password: %w", err)
	}
	_, err = s.DB.Exec(ctx, `
		INSERT INTO users (username, password_hash, must_change_password)
		VALUES ($1, $2, true)
		ON CONFLICT (username) DO NOTHING`, DefaultAdminUsername, hash)
	if err != nil {
		return fmt.Errorf("insert default admin: %w", err)
	}
	return nil
}

// UserByUsername returns the user with the given username, or
// ErrInvalidCredentials if none exists.
func (s Store) UserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
		SELECT id, username, password_hash, must_change_password,
		       twofa_secret, twofa_status, twofa_last_token
		FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.MustChangePassword,
			&u.TwoFASecret, &u.TwoFAStatus, &u.TwoFALastToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup user: %w", err)
	}
	return u, nil
}

// UserByID returns the user with the given id.
func (s Store) UserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
		SELECT id, username, password_hash, must_change_password,
		       twofa_secret, twofa_status, twofa_last_token
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.MustChangePassword,
			&u.TwoFASecret, &u.TwoFAStatus, &u.TwoFALastToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup user: %w", err)
	}
	return u, nil
}

// PrepareTwoFA stores a newly generated TOTP secret on the user row without
// enabling 2FA yet (twofa_status stays false until SaveTwoFA verifies the
// user actually configured their authenticator app).
func (s Store) PrepareTwoFA(ctx context.Context, userID int64, secret string) error {
	_, err := s.DB.Exec(ctx, `UPDATE users SET twofa_secret = $1, updated_at = now() WHERE id = $2`, secret, userID)
	if err != nil {
		return fmt.Errorf("prepare 2fa: %w", err)
	}
	return nil
}

// EnableTwoFA flips twofa_status on for the user, after the caller has
// verified a token against the just-prepared secret.
func (s Store) EnableTwoFA(ctx context.Context, userID int64) error {
	_, err := s.DB.Exec(ctx, `UPDATE users SET twofa_status = true, updated_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("enable 2fa: %w", err)
	}
	return nil
}

// DisableTwoFA flips twofa_status off. The secret is left in place (as
// Kuma does) rather than cleared, so re-enabling later is simple; it is
// inert while twofa_status is false.
func (s Store) DisableTwoFA(ctx context.Context, userID int64) error {
	_, err := s.DB.Exec(ctx, `UPDATE users SET twofa_status = false, updated_at = now() WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("disable 2fa: %w", err)
	}
	return nil
}

// SetTwoFALastToken records the most recently accepted TOTP token, used to
// reject an immediate replay of the same token within its validity window.
func (s Store) SetTwoFALastToken(ctx context.Context, userID int64, token string) error {
	_, err := s.DB.Exec(ctx, `UPDATE users SET twofa_last_token = $1 WHERE id = $2`, token, userID)
	if err != nil {
		return fmt.Errorf("set twofa last token: %w", err)
	}
	return nil
}

// CreateSession issues and persists a new session for the given user,
// returning the plaintext token to be stored in the client's cookie. Only
// the token's hash is written to the database.
func (s Store) CreateSession(ctx context.Context, userID int64) (token string, expiresAt time.Time, err error) {
	token, err = NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().Add(SessionTTL)
	_, err = s.DB.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)`, HashToken(token), userID, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert session: %w", err)
	}
	return token, expiresAt, nil
}

// SessionUser resolves a raw session token to its owning user, if the
// session exists and has not expired.
func (s Store) SessionUser(ctx context.Context, token string) (User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
		SELECT u.id, u.username, u.password_hash, u.must_change_password
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, HashToken(token)).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.MustChangePassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, errors.New("session not found or expired")
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup session: %w", err)
	}
	return u, nil
}

// DeleteSession invalidates a session token (logout).
func (s Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, HashToken(token))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// PruneExpired removes expired sessions. Intended to be called
// periodically so the sessions table does not grow unbounded.
func (s Store) PruneExpired(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}

// pendingTwoFATTL is how long a pending-2FA token (issued after a correct
// password but before the TOTP step) remains valid.
const pendingTwoFATTL = 5 * time.Minute

// CreatePendingTwoFA issues a short-lived token identifying a user who has
// passed the password step of login and now needs to submit a valid TOTP
// token to receive a real session.
func (s Store) CreatePendingTwoFA(ctx context.Context, userID int64) (token string, err error) {
	token, err = NewToken()
	if err != nil {
		return "", err
	}
	_, err = s.DB.Exec(ctx, `
		INSERT INTO twofa_pending (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)`, HashToken(token), userID, time.Now().Add(pendingTwoFATTL))
	if err != nil {
		return "", fmt.Errorf("insert pending 2fa: %w", err)
	}
	return token, nil
}

// ResolvePendingTwoFA returns the user a pending-2FA token was issued for,
// if it exists and has not expired. It does not consume the token.
func (s Store) ResolvePendingTwoFA(ctx context.Context, token string) (User, error) {
	var u User
	err := s.DB.QueryRow(ctx, `
		SELECT u.id, u.username, u.password_hash, u.must_change_password,
		       u.twofa_secret, u.twofa_status, u.twofa_last_token
		FROM twofa_pending p
		JOIN users u ON u.id = p.user_id
		WHERE p.token_hash = $1 AND p.expires_at > now()`, HashToken(token)).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.MustChangePassword,
			&u.TwoFASecret, &u.TwoFAStatus, &u.TwoFALastToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, errors.New("pending 2fa token not found or expired")
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup pending 2fa: %w", err)
	}
	return u, nil
}

// ConsumePendingTwoFA deletes a pending-2FA token so it cannot be reused,
// whether the TOTP step succeeded or failed.
func (s Store) ConsumePendingTwoFA(ctx context.Context, token string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM twofa_pending WHERE token_hash = $1`, HashToken(token))
	return err
}
