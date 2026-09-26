// Package secrets is Phase 0.2 of the RoutingNMS build blueprint: encryption
// at rest for secrets this codebase already stores in plaintext.
//
// Scope, stated plainly rather than silently narrowed: migration
// 0017_notification_channels.sql's own header comment already flags this
// exact gap -- "Secrets live in `config` JSON and should be encrypted at
// rest; this schema only stores the container" -- so this increment closes
// that specific, already-documented debt: notification_channels.config
// (webhook URLs, bot tokens, API keys, SMTP passwords, and every other
// per-provider secret field across the ~20 notification providers this
// codebase supports) is now encrypted at rest when a key is configured.
//
// It does NOT also encrypt devices.snmp_community/snmp_auth_password/
// snmp_priv_password or the equivalent OLT config columns in this pass --
// those are read from six different packages (devices, olt, topology,
// workspacetopology, discovery), each of which would need auditing to
// confirm it decrypts before use rather than passing ciphertext straight to
// gosnmp. That is real work, not a search-and-replace, so it is the
// explicit next increment (0.2b), matching the same one-surface-first
// pattern Feature 0.1 used for RBAC (schema + middleware + one proof route,
// not a full endpoint retrofit in the same pass).
//
// Encryption is opt-in and fails safe toward "unchanged": with no
// ROUTINGNMS_SECRETS_KEY set, Cipher is the zero value and every
// Encrypt/EncryptJSON call is a pass-through no-op -- existing deployments
// that don't set the env var see zero behavior change, matching this
// project's "additive, never disruptive to existing configs" rule. Once an
// operator sets the key, new/updated rows get encrypted going forward;
// existing plaintext rows keep reading correctly (Decrypt/DecryptJSON only
// treat a value as ciphertext if it carries this package's own prefix/
// wrapper) and are re-encrypted the next time they're saved -- a
// write-through migration, not a bulk rewrite that could corrupt data if
// the key were ever wrong.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// envKey is the environment variable an operator sets to enable encryption:
// a base64-encoded 32-byte (AES-256) key, e.g. `openssl rand -base64 32`.
const envKey = "ROUTINGNMS_SECRETS_KEY"

// prefix marks a Cipher-produced ciphertext string, distinguishing it from
// plaintext so Decrypt can tell old rows from new ones without a schema
// version column.
const prefix = "enc:v1:"

// Cipher wraps an optional AES-256-GCM key. The zero value (no key) makes
// every method a no-op pass-through -- see the package doc comment for why
// that's the deliberate, safe default.
type Cipher struct {
	key []byte // nil/empty = disabled
}

// LoadKeyFromEnv reads ROUTINGNMS_SECRETS_KEY. An unset/empty value returns
// a disabled Cipher (nil error) -- this is the expected state for any
// deployment that hasn't opted in yet, not a misconfiguration. A malformed
// value (not base64, or not 32 bytes once decoded) is an error: better to
// fail startup loudly than silently store plaintext under a broken key.
func LoadKeyFromEnv() (Cipher, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return Cipher{}, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return Cipher{}, fmt.Errorf("%s must be base64-encoded: %w", envKey, err)
	}
	if len(key) != 32 {
		return Cipher{}, fmt.Errorf("%s must decode to 32 bytes (AES-256), got %d", envKey, len(key))
	}
	return Cipher{key: key}, nil
}

// Enabled reports whether a key is configured.
func (c Cipher) Enabled() bool { return len(c.key) == 32 }

// Encrypt returns plaintext unchanged when disabled. An empty string is
// never encrypted either way (keeps "not set" distinguishable from "set to
// an encrypted empty value", and avoids paying a nonce+tag for nothing).
// Otherwise returns "enc:v1:<base64 nonce||ciphertext||tag>" via AES-256-GCM
// with a fresh random nonce per call.
func (c Cipher) Encrypt(plaintext string) (string, error) {
	if !c.Enabled() || plaintext == "" {
		return plaintext, nil
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("secrets: init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secrets: init gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secrets: generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. A value with no "enc:v1:" prefix is returned
// as-is -- this is what makes reading old, pre-encryption plaintext rows
// safe even after a key is configured. A prefixed value with no key
// configured is an error (the data is real ciphertext; there's nothing
// sane to return).
func (c Cipher) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	if !c.Enabled() {
		return "", fmt.Errorf("secrets: value is encrypted but %s is not set", envKey)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", fmt.Errorf("secrets: decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("secrets: init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secrets: init gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("secrets: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("secrets: decrypt: %w (wrong key, or data corrupted)", err)
	}
	return string(plain), nil
}

// encJSONWrapper is the on-disk shape of an encrypted JSON blob: a single
// sentinel key so DecryptJSON can tell "this JSONB value is one opaque
// ciphertext" apart from "this JSONB value is the real, structured plain
// JSON" (the pre-encryption shape) without a schema/version column.
type encJSONWrapper struct {
	Enc string `json:"__enc"`
}

// EncryptJSON marshals v to JSON, then -- only when a key is configured --
// re-wraps the whole marshaled blob as one ciphertext string under
// {"__enc":"enc:v1:..."}. Disabled Ciphers return plain json.Marshal(v)
// unchanged, so a deployment that never sets the key sees byte-identical
// storage to before this feature existed.
//
// Whole-payload encryption (rather than encrypting individual known field
// names like "password"/"token"/"api_key") is deliberate: this codebase's
// notification_channels.config alone spans ~20 provider types with
// differently-named secret fields (bot_token, api_key, routing_key,
// send_key, secret_key, access_token, auth_token, ...), so a per-field
// allowlist would need constant upkeep and could silently miss one. A
// config blob has no non-secret fields worth reading without decrypting
// anyway, so there's no cost to treating the whole thing as sensitive.
func (c Cipher) EncryptJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("secrets: marshal: %w", err)
	}
	if !c.Enabled() {
		return raw, nil
	}
	enc, err := c.Encrypt(string(raw))
	if err != nil {
		return nil, err
	}
	return json.Marshal(encJSONWrapper{Enc: enc})
}

// DecryptJSON reverses EncryptJSON into v. Transparently handles both the
// {"__enc":...} wrapper (new, encrypted rows) and legacy plain JSON (rows
// written before a key was ever configured, or while it's disabled) --
// callers don't need to know which shape a given row is in.
func (c Cipher) DecryptJSON(raw []byte, v any) error {
	var wrapper encJSONWrapper
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Enc != "" {
		plain, err := c.Decrypt(wrapper.Enc)
		if err != nil {
			return err
		}
		return json.Unmarshal([]byte(plain), v)
	}
	return json.Unmarshal(raw, v)
}
