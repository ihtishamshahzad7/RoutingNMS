// Package auth implements RoutingNMS operator authentication: password
// hashing, session issuance/verification and HTTP handlers/middleware.
//
// Password hashing uses PBKDF2-HMAC-SHA256 (crypto/pbkdf2, part of the Go
// 1.24 standard library) rather than a third-party dependency such as
// bcrypt, so authentication has zero extra module dependencies and cannot be
// broken by a blocked module proxy during `go mod download` in production.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 210_000
	pbkdf2KeyLength  = 32
	saltLength       = 16
	hashPrefix       = "pbkdf2-sha256"
)

// HashPassword derives a salted PBKDF2-HMAC-SHA256 hash for the given
// plaintext password and encodes it as a self-describing string:
//
//	pbkdf2-sha256$<iterations>$<salt-hex>$<derived-key-hex>
//
// The iteration count is stored alongside the hash so it can be increased in
// the future without invalidating previously issued hashes.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	return fmt.Sprintf("%s$%d$%s$%s", hashPrefix, pbkdf2Iterations, hex.EncodeToString(salt), hex.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the previously encoded
// hash produced by HashPassword. Comparison of the derived key is constant
// time to avoid leaking timing information.
func VerifyPassword(encodedHash, password string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 4 || parts[0] != hashPrefix {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(want, got) == 1
}

// NewToken returns a cryptographically random, URL-safe session token.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// HashToken returns the SHA-256 hex digest of a session token, which is what
// gets persisted to the database. Storing only the hash means a database
// leak alone cannot be used to forge a valid session cookie.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
