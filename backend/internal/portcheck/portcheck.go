// Package portcheck implements the "Port/Service Check" monitor type
// (6-page rebuild, item 2.1): either a plain TCP connect test, or -- for
// protocol http/https -- a real HTTP(S) GET against host:port+path with
// status-code validation. Mirrors the narrow scope of internal/sshcheck
// and internal/telnetcheck (a reachability probe, not a login/handshake),
// and reuses internal/httpcheck's own accepted-status-codes matching
// (exact codes or "lo-hi" ranges) for the http/https protocol so the two
// HTTP-validating monitor types can never silently drift apart.
package portcheck

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/httpcheck"
)

// Result is the outcome of one Port/Service check.
type Result struct {
	Reachable  bool
	LatencyMS  float64
	StatusCode int // 0 for protocol "tcp" (no HTTP response to report)
	Error      string
}

// Check runs one probe. For protocol "tcp" (the default) it is a plain
// TCP-connect test; for "http"/"https" it is a real GET request to
// protocol://host:port/path, validated against acceptedCodes the same way
// the HTTP(s) monitor type validates its own accepted status codes.
func Check(ctx context.Context, protocol, host string, port int, path string, acceptedCodes []string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if port <= 0 {
		port = 80
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))

	switch protocol {
	case "http", "https":
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		url := protocol + "://" + net.JoinHostPort(host, strconv.Itoa(port)) + path
		r := httpcheck.Check(ctx, url, httpcheck.Options{
			Method:              http.MethodGet,
			AcceptedStatusCodes: acceptedCodes,
			MaxRedirects:        10,
			Timeout:             timeout,
		})
		return Result{Reachable: r.Reachable, LatencyMS: r.LatencyMS, StatusCode: r.StatusCode, Error: r.Error}
	default:
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		start := time.Now()
		dialer := net.Dialer{Timeout: timeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		latency := time.Since(start)
		if err != nil {
			return Result{Error: err.Error(), LatencyMS: float64(latency.Milliseconds())}
		}
		defer conn.Close()
		return Result{Reachable: true, LatencyMS: float64(latency.Milliseconds())}
	}
}
