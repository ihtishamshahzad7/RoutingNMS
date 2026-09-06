// Package httpcheck implements the HTTP(S)+keyword monitor type ported from
// the user's previous Uptime Kuma deployment: fetch a URL, check the status
// code and (optionally) that the response body contains a keyword, and --
// for https:// targets -- report how many days remain until the TLS
// certificate expires, so an operator can be warned before it lapses.
package httpcheck

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"
)

// Result is the outcome of one HTTP check.
type Result struct {
	Reachable        bool
	StatusCode       int
	LatencyMS        float64
	KeywordMatched   *bool      // nil when no keyword was configured
	CertExpiryInDays *int       // nil for a plain http:// URL, or if the cert couldn't be inspected
	CertChain        []CertLink // nil for a plain http:// URL, or if the cert couldn't be inspected
	Error            string
}

// CertLink describes one certificate in the TLS chain presented by the
// server, mirroring (a simplified version of) the certificate details Kuma
// captures via Node's tls.getPeerCertificate({detailed: true}) and displays
// on its monitor detail page.
type CertLink struct {
	CertType          string    `json:"certType"` // "server" for the leaf (index 0), "intermediate CA" for the rest
	Subject           string    `json:"subject"`  // pkix.Name.CommonName, falling back to the full String() if blank
	Issuer            string    `json:"issuer"`
	ValidFrom         time.Time `json:"validFrom"`
	ValidTo           time.Time `json:"validTo"`
	FingerprintSHA256 string    `json:"fingerprintSha256"` // hex-encoded sha256(cert.Raw), Kuma's own fingerprint scheme
}

// Check fetches url, verifies the status code and (if keyword is non-empty)
// that the response body contains it, and reports TLS cert expiry for
// https:// targets. It never returns a Go error -- every failure mode
// (unreachable, wrong status, missing keyword) is reported in Result so a
// caller can always record a metric sample.
func Check(ctx context.Context, url string, expectedStatus int, keyword string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Error: err.Error()}
	}
	req.Header.Set("User-Agent", "RoutingNMS-http-monitor/1.0")

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Error: err.Error(), LatencyMS: float64(latency.Milliseconds())}
	}
	defer resp.Body.Close()

	result := Result{
		StatusCode: resp.StatusCode,
		LatencyMS:  float64(latency.Milliseconds()),
		Reachable:  expectedStatus == 0 || resp.StatusCode == expectedStatus,
	}

	if keyword != "" {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap at 1MB, matching Kuma's own guard
		matched := readErr == nil && strings.Contains(string(body), keyword)
		result.KeywordMatched = &matched
		if !matched {
			result.Reachable = false
		}
	}

	if resp.TLS != nil {
		if days := certExpiryDays(resp.TLS); days != nil {
			result.CertExpiryInDays = days
		}
		result.CertChain = certChain(resp.TLS)
	}

	return result
}

func certExpiryDays(state *tls.ConnectionState) *int {
	if state == nil || len(state.PeerCertificates) == 0 {
		return nil
	}
	// The leaf certificate is always first.
	expiry := state.PeerCertificates[0].NotAfter
	days := int(time.Until(expiry).Hours() / 24)
	return &days
}

// certChain builds a Kuma-style chain summary from the certificates the
// server actually presented on the wire (state.PeerCertificates), NOT from
// state.VerifiedChains. We deliberately use PeerCertificates: it is the
// direct Go equivalent of what Node's tls socket exposes as
// getPeerCertificate()/issuerCertificate (what the server sent), whereas
// VerifiedChains reflects Go's own trust-store validation and can omit
// certs the server sent or substitute differently-sourced ones -- using
// PeerCertificates keeps the "what does this server present" semantics
// Kuma's panel is built around.
//
// Known simplification vs. Kuma: Node's chain walk terminates at a
// self-signed root (from the OS trust store or the chain itself) and the
// UI implicitly treats the last link as the root CA. Go's PeerCertificates
// only contains what the server sent (almost never the root CA itself, per
// TLS convention), and Go's stdlib has no lightweight way to fetch/label a
// system root cert by AuthorityKeyId. So every certificate here is labeled
// "server" (index 0) or "intermediate CA" (the rest) -- we never label a
// "root CA" the way Kuma's UI does. This is a cosmetic gap, not a
// correctness one: the leaf validity dates driving CertExpiryInDays are
// unaffected.
func certChain(state *tls.ConnectionState) []CertLink {
	if state == nil || len(state.PeerCertificates) == 0 {
		return nil
	}
	chain := make([]CertLink, 0, len(state.PeerCertificates))
	for i, cert := range state.PeerCertificates {
		certType := "intermediate CA"
		if i == 0 {
			certType = "server"
		}
		subject := cert.Subject.CommonName
		if subject == "" {
			subject = cert.Subject.String()
		}
		issuer := cert.Issuer.CommonName
		if issuer == "" {
			issuer = cert.Issuer.String()
		}
		sum := sha256.Sum256(cert.Raw)
		chain = append(chain, CertLink{
			CertType:          certType,
			Subject:           subject,
			Issuer:            issuer,
			ValidFrom:         cert.NotBefore,
			ValidTo:           cert.NotAfter,
			FingerprintSHA256: hex.EncodeToString(sum[:]),
		})
	}
	return chain
}
