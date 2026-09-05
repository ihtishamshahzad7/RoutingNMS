// Package sshcheck implements the SSH reachability monitor type: TCP-connect
// to the configured port (default 22) and, if a banner keyword is
// configured, read the identification banner an SSH server sends
// immediately on connect (RFC 4253 4.2, e.g. "SSH-2.0-OpenSSH_9.6") and
// verify it contains the keyword. No SSH handshake/auth is attempted --
// this is a reachability check, not a login check, mirroring how
// internal/dnscheck and internal/httpcheck are narrowly scoped.
package sshcheck

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"
)

// Result is the outcome of one SSH reachability check.
type Result struct {
	Reachable     bool
	Banner        string
	LatencyMS     float64
	BannerMatched *bool // nil when no banner keyword was configured
	Error         string
}

// Check dials host:port, and -- if bannerKeyword is non-empty -- reads the
// server's identification banner and verifies it contains bannerKeyword.
func Check(ctx context.Context, host string, port int, bannerKeyword string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if port <= 0 {
		port = 22
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
	banner := strings.TrimRight(string(buf[:n]), "\r\n")
	res.Banner = banner
	matched := strings.Contains(banner, keyword)
	res.BannerMatched = &matched
	if !matched {
		res.Reachable = false
		res.Error = "banner did not contain expected keyword"
	}
	return res
}
