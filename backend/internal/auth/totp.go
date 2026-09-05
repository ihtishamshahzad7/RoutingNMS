package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP (RFC 6238) implemented against only the Go standard library
// (crypto/hmac, crypto/sha1, encoding/base32, crypto/rand, time) — no
// third-party TOTP/OTP module, matching this codebase's zero-extra-
// dependency convention for authentication primitives (see password.go).
//
// Parameters mirror the near-universal authenticator-app default, and
// Kuma's own choice: HMAC-SHA1, 30 second step, 6 digit codes.
const (
	totpStepSeconds = 30
	totpDigits      = 6
	totpSecretBytes = 20 // 160 bits, standard for HMAC-SHA1 TOTP secrets
	// totpSkewSteps is how many steps before/after "now" are accepted,
	// matching Kuma's window of 1 (i.e. now-30s, now, now+30s).
	totpSkewSteps = 1
)

// base32NoPad is the RFC 4648 base32 alphabet without padding, the
// conventional encoding for TOTP secrets (what authenticator apps expect
// when a secret is typed in or embedded in an otpauth:// URI).
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret returns a new random base32-encoded TOTP secret.
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, totpSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32NoPad.EncodeToString(buf), nil
}

// totpCode computes the RFC 6238 TOTP code for the given base32 secret and
// 30-second-step counter.
func totpCode(secret string, counter uint64) (string, error) {
	key, err := base32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	code %= mod
	return fmt.Sprintf("%0*d", totpDigits, code), nil
}

// VerifyTOTP reports whether token is valid for secret at the given time,
// allowing +/- totpSkewSteps steps of clock skew. On success it returns the
// exact token string that matched (always equal to the submitted token,
// returned for caller convenience when recording twofa_last_token).
func VerifyTOTP(secret, token string, now time.Time) bool {
	token = strings.TrimSpace(token)
	if len(token) != totpDigits {
		return false
	}
	counter := uint64(now.Unix() / totpStepSeconds)
	for skew := -totpSkewSteps; skew <= totpSkewSteps; skew++ {
		c := counter
		if skew < 0 {
			if uint64(-skew) > c {
				continue
			}
			c -= uint64(-skew)
		} else {
			c += uint64(skew)
		}
		want, err := totpCode(secret, c)
		if err != nil {
			return false
		}
		if want == token {
			return true
		}
	}
	return false
}

// TOTPURI builds an otpauth:// URI for QR-code display in an authenticator
// app, per the de facto Google Authenticator Key URI format.
func TOTPURI(issuer, accountName, secret string) string {
	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, accountName))
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", totpDigits))
	q.Set("period", fmt.Sprintf("%d", totpStepSeconds))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, q.Encode())
}
