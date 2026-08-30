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
	err := s.DB.QueryRow(ctx, `SELECT id, username, password_hash, must_change_password FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.MustChangePassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup user: %w", err)
	}
	return u, nil
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
