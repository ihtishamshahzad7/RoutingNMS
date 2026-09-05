// Package apikeys implements RoutingNMS API keys: user-scoped bearer
// credentials an operator can issue for scripts/integrations, as an
// alternative to the session cookie.
//
// Mirrors Uptime Kuma's own API key design: the presented key is shaped
// "rns_<row id>_<random secret>" ("rns" for RoutingNMS, matching Kuma's own
// "uk" prefix convention). Only the row id travels in cleartext; the
// secret portion is hashed (reusing this codebase's existing PBKDF2-SHA256
// password hash from internal/auth -- no new hashing dependency) before
// being stored, so a database leak alone cannot be used to forge a key.
// Verification looks the row up by id first (an O(1) index lookup, not a
// full-table scan comparing the presented secret against every stored
// hash -- the same performance concern Kuma's own implementation avoids),
// then cheaply rejects an inactive or expired key before paying for the
// comparatively expensive hash comparison.
package apikeys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/auth"
)

// KeyPrefix identifies a RoutingNMS API key, mirroring Kuma's "uk<id>_<secret>"
// shape with a RoutingNMS-specific prefix.
const KeyPrefix = "rns"

// secretBytes is the length, in random bytes, of the key's secret portion
// before hex-encoding (32 hex chars).
const secretBytes = 16

var (
	// ErrInvalidKey is returned by Verify for any key that is malformed,
	// unknown, inactive, expired, or whose secret does not match.
	// Intentionally generic: callers must not distinguish these cases in
	// their response, to avoid leaking which failure mode occurred.
	ErrInvalidKey = errors.New("invalid api key")
)

// Record is one issued API key, as stored (never includes the raw secret).
type Record struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"userId"`
	Name        string     `json:"name"`
	Active      bool       `json:"active"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedDate time.Time  `json:"createdDate"`
}

// Repository persists API keys in PostgreSQL.
type Repository struct{ DB *pgxpool.Pool }

// generateSecret returns a random hex-encoded secret for the key portion
// after the row id.
func generateSecret() (string, error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Create issues a new API key for userID, returning the persisted record
// and the one-time plaintext key string (shaped "rns_<id>_<secret>") that
// must be shown to the caller immediately -- it is never recoverable again,
// since only its hash is stored.
func (r Repository) Create(ctx context.Context, userID int64, name string, expiresAt *time.Time) (Record, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Record{}, "", errors.New("name is required")
	}
	secret, err := generateSecret()
	if err != nil {
		return Record{}, "", err
	}
	hash, err := auth.HashPassword(secret)
	if err != nil {
		return Record{}, "", fmt.Errorf("hash api key secret: %w", err)
	}

	var rec Record
	rec.UserID = userID
	rec.Name = name
	rec.ExpiresAt = expiresAt
	err = r.DB.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, name, secret_hash, active, expires_at)
		VALUES ($1, $2, $3, true, $4)
		RETURNING id, active, created_date`, userID, name, hash, expiresAt).
		Scan(&rec.ID, &rec.Active, &rec.CreatedDate)
	if err != nil {
		return Record{}, "", fmt.Errorf("insert api key: %w", err)
	}

	key := fmt.Sprintf("%s_%d_%s", KeyPrefix, rec.ID, secret)
	return rec, key, nil
}

// List returns every API key belonging to userID, newest first.
func (r Repository) List(ctx context.Context, userID int64) ([]Record, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT id, user_id, name, active, expires_at, created_date
		FROM api_keys WHERE user_id = $1 ORDER BY created_date DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	out := []Record{}
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.Active, &rec.ExpiresAt, &rec.CreatedDate); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// SetActive enables or disables (revokes) an API key. It only affects a
// key owned by userID, so one user cannot disable another's key.
func (r Repository) SetActive(ctx context.Context, userID, id int64, active bool) error {
	tag, err := r.DB.Exec(ctx, `UPDATE api_keys SET active = $1 WHERE id = $2 AND user_id = $3`, active, id, userID)
	if err != nil {
		return fmt.Errorf("update api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("api key not found")
	}
	return nil
}

// Delete permanently removes an API key. It only affects a key owned by
// userID.
func (r Repository) Delete(ctx context.Context, userID, id int64) error {
	tag, err := r.DB.Exec(ctx, `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("api key not found")
	}
	return nil
}

// parseKey splits a presented "rns_<id>_<secret>" key into its row id and
// secret portion.
func parseKey(key string) (id int64, secret string, ok bool) {
	if !strings.HasPrefix(key, KeyPrefix+"_") {
		return 0, "", false
	}
	rest := key[len(KeyPrefix)+1:]
	sep := strings.IndexByte(rest, '_')
	if sep < 0 {
		return 0, "", false
	}
	idStr, secret := rest[:sep], rest[sep+1:]
	if secret == "" {
		return 0, "", false
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	return id, secret, true
}

// Verify resolves a presented API key string to its owning user id.
//
// It parses the row id out of the key up front and looks up that single
// row by primary key -- deliberately not a full-table scan comparing the
// presented secret against every stored hash, which would get slower as
// more keys are issued and is the exact cost Kuma's own key format is
// designed to avoid. Cheap checks (active, not expired) run before the
// comparatively expensive password-hash comparison.
func (r Repository) Verify(ctx context.Context, key string) (userID int64, err error) {
	id, secret, ok := parseKey(key)
	if !ok {
		return 0, ErrInvalidKey
	}

	var (
		storedUserID int64
		hash         string
		active       bool
		expiresAt    *time.Time
	)
	err = r.DB.QueryRow(ctx, `
		SELECT user_id, secret_hash, active, expires_at FROM api_keys WHERE id = $1`, id).
		Scan(&storedUserID, &hash, &active, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInvalidKey
	}
	if err != nil {
		return 0, fmt.Errorf("lookup api key: %w", err)
	}
	if !active {
		return 0, ErrInvalidKey
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return 0, ErrInvalidKey
	}
	if !auth.VerifyPassword(hash, secret) {
		return 0, ErrInvalidKey
	}
	return storedUserID, nil
}
