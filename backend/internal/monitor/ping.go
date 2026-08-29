package monitor

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Ping performs a TCP reachability probe. It is intentionally unprivileged so
// the NMS worker can run as a non-root user. ICMP support can be added through
// a dedicated probe implementation without changing the domain model.
func Ping(ctx context.Context, address string, timeout time.Duration) ProbeResult {
	started := time.Now()
	result := ProbeResult{Address: address, CheckedAt: started}

	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(address, "80")
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		result.Error = fmt.Sprintf("probe failed: %v", err)
		result.Latency = time.Since(started)
		return result
	}
	_ = conn.Close()
	result.Reachable = true
	result.Latency = time.Since(started)
	return result
}
