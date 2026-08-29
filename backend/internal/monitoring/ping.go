package monitoring

import (
    "context"
    "fmt"
    "net"
    "time"
)

type PingResult struct {
    Address string `json:"address"`
    Reachable bool `json:"reachable"`
    LatencyMs float64 `json:"latencyMs"`
    Error string `json:"error,omitempty"`
}

// TCPProbe provides an unprivileged reachability/latency probe. It avoids
// requiring CAP_NET_RAW in the NMS service container. ICMP can be added as a
// privileged probe implementation behind the same interface.
func TCPProbe(ctx context.Context, address string, port string, timeout time.Duration) PingResult {
    result := PingResult{Address: address}
    if timeout <= 0 { timeout = 3 * time.Second }
    started := time.Now()
    d := net.Dialer{Timeout: timeout}
    conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(address, port))
    if err != nil { result.Error = fmt.Sprintf("probe failed: %v", err); return result }
    _ = conn.Close()
    result.Reachable = true
    result.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
    return result
}
