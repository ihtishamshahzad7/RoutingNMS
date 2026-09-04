// Package httpcheck implements the HTTP(S)+keyword monitor type ported from
// the user's previous Uptime Kuma deployment: fetch a URL, check the status
// code and (optionally) that the response body contains a keyword, and --
// for https:// targets -- report how many days remain until the TLS
// certificate expires, so an operator can be warned before it lapses.
package httpcheck

import (
	"context"
	"crypto/tls"
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
	KeywordMatched   *bool // nil when no keyword was configured
	CertExpiryInDays *int  // nil for a plain http:// URL, or if the cert couldn't be inspected
	Error            string
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
