// Package httpcheck implements the HTTP(S)+keyword monitor type ported from
// the user's previous Uptime Kuma deployment: fetch a URL, check the status
// code and (optionally) that the response body contains a keyword, and --
// for https:// targets -- report how many days remain until the TLS
// certificate expires, so an operator can be warned before it lapses.
package httpcheck

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
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

// Options carries the Kuma-parity HTTP(s) monitor fields beyond the plain
// GET+status-code+keyword check this package originally implemented --
// method, headers, body, accepted status codes, follow-redirect count, and
// ignore-TLS-error, read directly from the user's real Uptime Kuma source
// (src/pages/EditMonitor.vue's "HTTP Options" section) rather than guessed:
// Method is one of GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS (default GET);
// Headers is a raw JSON object string, e.g. {"HeaderName": "HeaderValue"}
// (Kuma's own headersPlaceholder shape); Body is sent as-is with a
// Content-Type set from BodyEncoding ("json" -> application/json, "xml" ->
// text/xml); AcceptedStatusCodes is a list of exact codes ("404") or ranges
// ("200-299"), same as Kuma's acceptedStatusCodeOptions; MaxRedirects is
// Kuma's "Max. Redirects" field (0 disables following); IgnoreTLS skips
// certificate verification, matching Kuma's "ignoreTls" checkbox.
type Options struct {
	Method              string
	Headers             string
	Body                string
	BodyEncoding        string
	AcceptedStatusCodes []string
	MaxRedirects        int
	IgnoreTLS           bool
	// ExpectedStatus is the pre-Kuma-parity fallback exact-status check,
	// used only when AcceptedStatusCodes is empty -- keeps any HTTP check
	// configured before this feature working unchanged.
	ExpectedStatus int
	Keyword        string
	Timeout        time.Duration
}

// Check fetches url per opts (method/headers/body/accepted status codes/
// redirects/TLS verification) and reports TLS cert expiry for https://
// targets. It never returns a Go error -- every failure mode (unreachable,
// wrong status, missing keyword) is reported in Result so a caller can
// always record a metric sample.
func Check(ctx context.Context, url string, opts Options) Result {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}
	maxRedirects := opts.MaxRedirects

	transport := http.DefaultTransport
	if opts.IgnoreTLS {
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // #nosec G402 -- explicit opt-in, mirrors Kuma's own "Ignore TLS Error" checkbox
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if strings.TrimSpace(opts.Body) != "" {
		bodyReader = bytes.NewBufferString(opts.Body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
	if err != nil {
		return Result{Error: err.Error()}
	}
	req.Header.Set("User-Agent", "RoutingNMS-http-monitor/1.0")
	if bodyReader != nil {
		switch opts.BodyEncoding {
		case "xml":
			req.Header.Set("Content-Type", "text/xml")
		default:
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if strings.TrimSpace(opts.Headers) != "" {
		var headers map[string]string
		if jsonErr := json.Unmarshal([]byte(opts.Headers), &headers); jsonErr == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
		// A malformed headers field is not fatal -- matches Kuma's own
		// tolerance of a monitor being saved mid-edit; the request still
		// goes out with whatever headers did parse.
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Error: err.Error(), LatencyMS: float64(latency.Milliseconds())}
	}
	defer resp.Body.Close()

	reachable := statusAccepted(resp.StatusCode, opts.AcceptedStatusCodes, opts.ExpectedStatus)
	result := Result{
		StatusCode: resp.StatusCode,
		LatencyMS:  float64(latency.Milliseconds()),
		Reachable:  reachable,
	}

	if opts.Keyword != "" {
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap at 1MB, matching Kuma's own guard
		matched := readErr == nil && strings.Contains(string(respBody), opts.Keyword)
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

// statusAccepted checks status against Kuma's accepted-status-codes scheme:
// each pattern is either an exact code ("404") or a range ("200-299"). An
// empty acceptedCodes list falls back to the pre-Kuma-parity exact
// expectedStatus check (0 meaning "any status is fine"), so a device
// configured before this feature keeps behaving exactly as it did.
// StatusAccepted is the exported form of statusAccepted, for other check
// types that validate an HTTP(S) response against this same
// accepted-status-codes convention (e.g. internal/portcheck's http/https
// protocol, 6-page rebuild item 2.1) instead of re-implementing the
// exact-code-or-"lo-hi"-range matching rules.
func StatusAccepted(status int, acceptedCodes []string, expectedStatus int) bool {
	return statusAccepted(status, acceptedCodes, expectedStatus)
}

func statusAccepted(status int, acceptedCodes []string, expectedStatus int) bool {
	if len(acceptedCodes) == 0 {
		return expectedStatus == 0 || status == expectedStatus
	}
	for _, pattern := range acceptedCodes {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if lo, hi, ok := parseStatusRange(pattern); ok {
			if status >= lo && status <= hi {
				return true
			}
			continue
		}
		if n, err := strconv.Atoi(pattern); err == nil && status == n {
			return true
		}
	}
	return false
}

func parseStatusRange(pattern string) (lo, hi int, ok bool) {
	parts := strings.SplitN(pattern, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	lo, errLo := strconv.Atoi(strings.TrimSpace(parts[0]))
	hi, errHi := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errLo != nil || errHi != nil {
		return 0, 0, false
	}
	return lo, hi, true
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
