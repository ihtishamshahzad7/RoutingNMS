// Package telnetcheck implements the Telnet reachability monitor type:
// TCP-connect to the configured port (default 23) and, if a banner keyword
// is configured, read whatever the server sends first -- IAC negotiation
// bytes and/or a login prompt banner -- and verify the printable portion
// contains the keyword. No login is attempted -- reachability only,
// mirroring sshcheck's scope.
package telnetcheck

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"
)

// Result is the outcome of one Telnet reachability check.
type Result struct {
	Reachable     bool
	Banner        string
	LatencyMS     float64
	BannerMatched *bool
	Error         string
}

// Check dials host:port, and -- if bannerKeyword is non-empty -- reads
// whatever the server sends first and verifies its printable text contains
// bannerKeyword.
func Check(ctx context.Context, host string, port int, bannerKeyword string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if port <= 0 {
		port = 23
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	latency := time.Since(start)
	if err != nil {
		return Result{Error: err.Error(), LatencyMS: float64(latency.Milliseconds())}
	}
	defer conn.Close()

	res := Result{Reachable: true, LatencyMS: float64(latency.Milliseconds())}

	keyword := strings.TrimSpace(bannerKeyword)
	if keyword == "" {
		return res
	}

	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 512)
	n, readErr := conn.Read(buf)
	if readErr != nil && n == 0 {
		matched := false
		res.BannerMatched = &matched
		res.Reachable = false
		res.Error = "no banner received: " + readErr.Error()
		return res
	}
	banner := printable(buf[:n])
	res.Banner = banner
	matched := strings.Contains(banner, keyword)
	res.BannerMatched = &matched
	if !matched {
		res.Reachable = false
		res.Error = "banner did not contain expected keyword"
	}
	return res
}

// printable strips Telnet IAC negotiation bytes (>= 0xF0, RFC 854) so a
// keyword match runs against the human-readable login-prompt text, not raw
// control bytes.
func printable(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == '\r' || c == '\n' || (c >= 0x20 && c < 0x7F) {
			out = append(out, c)
		}
	}
	return strings.TrimSpace(string(out))
}
