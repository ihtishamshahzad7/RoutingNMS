// Package traceroute implements an on-demand hop-by-hop path trace to a
// device, an "advanced" ping capability Uptime Kuma never offered (its ping
// monitor is reachability-only). Useful on an ISP network for diagnosing
// *where* a path breaks down, not just *that* it did.
package traceroute

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// Hop is one line of traceroute output.
type Hop struct {
	Number   int      `json:"number"`
	Address  string   `json:"address,omitempty"`  // empty when the hop timed out
	Hostname string   `json:"hostname,omitempty"` // reverse-DNS name, when the tool resolved one
	RTTMs    *float64 `json:"rttMs,omitempty"`
	TimedOut bool     `json:"timedOut"`
}

// Result is a full trace.
type Result struct {
	Address   string    `json:"address"`
	Hops      []Hop     `json:"hops"`
	RanAt     time.Time `json:"ranAt"`
	Error     string    `json:"error,omitempty"`
	RawOutput string    `json:"-"` // kept off the wire; useful for local debugging only
}

// Unix traceroute line, e.g.:
//
//	1  192.0.2.1 (192.0.2.1)  0.434 ms
//	2  router.example.com (203.0.113.1)  1.203 ms  1.198 ms  1.205 ms
//	3  * * *
//
// Windows tracert line, e.g.:
//
//	1     1 ms     1 ms     1 ms  192.168.1.1
//	2     *        *        *     Request timed out.
var (
	reUnixHop    = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
	reUnixHostIP = regexp.MustCompile(`^([^\s(]+)\s+\(([^)]+)\)`)
	reUnixIPOnly = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3}){3}|[0-9a-fA-F:]+)\b`)
	reUnixRTT    = regexp.MustCompile(`([0-9.]+)\s*ms`)
	reWinHop     = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
	reWinRTT     = regexp.MustCompile(`([0-9.]+)\s*ms`)
	reWinIP      = regexp.MustCompile(`(\d{1,3}(?:\.\d{1,3}){3})\s*$`)
)

// Run performs a traceroute to address (hostname or IP), capped at maxHops
// (default 20) with a per-run timeout. It shells out to the system
// traceroute/tracert binary, matching the same "unprivileged by default,
// exec the platform tool" approach as internal/ping.ExecPing.
func Run(ctx context.Context, address string, maxHops int) Result {
	res := Result{Address: address, RanAt: time.Now().UTC()}
	if maxHops <= 0 || maxHops > 64 {
		maxHops = 20
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "tracert", "-d", "-h", strconv.Itoa(maxHops), "-w", "2000", address)
	} else {
		cmd = exec.CommandContext(ctx, "traceroute", "-n", "-q", "1", "-w", "2", "-m", strconv.Itoa(maxHops), address)
	}
	out, err := cmd.CombinedOutput()
	text := string(out)
	res.RawOutput = text
	if err != nil && len(text) == 0 {
		res.Error = fmt.Sprintf("traceroute failed: %v", err)
		return res
	}

	if runtime.GOOS == "windows" {
		res.Hops = parseWindows(text)
	} else {
		res.Hops = parseUnix(text)
	}
	if len(res.Hops) == 0 && res.Error == "" {
		res.Error = "no hops parsed from traceroute output"
	}
	return res
}

func parseUnix(text string) []Hop {
	out := []Hop{}
	for _, line := range splitLines(text) {
		m := reUnixHop.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		num, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		rest := m[2]
		hop := Hop{Number: num}
		if hostIP := reUnixHostIP.FindStringSubmatch(rest); hostIP != nil {
			hop.Hostname, hop.Address = hostIP[1], hostIP[2]
			if hop.Hostname == hop.Address {
				hop.Hostname = ""
			}
		} else if ip := reUnixIPOnly.FindStringSubmatch(rest); ip != nil {
			hop.Address = ip[1]
		}
		if rtt := reUnixRTT.FindStringSubmatch(rest); rtt != nil {
			if v, err := strconv.ParseFloat(rtt[1], 64); err == nil {
				hop.RTTMs = &v
			}
		}
		hop.TimedOut = hop.Address == "" && hop.RTTMs == nil
		out = append(out, hop)
	}
	return out
}

func parseWindows(text string) []Hop {
	out := []Hop{}
	for _, line := range splitLines(text) {
		m := reWinHop.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		num, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		rest := m[2]
		hop := Hop{Number: num}
		if ip := reWinIP.FindStringSubmatch(rest); ip != nil {
			hop.Address = ip[1]
		}
		if rtt := reWinRTT.FindStringSubmatch(rest); rtt != nil {
			if v, err := strconv.ParseFloat(rtt[1], 64); err == nil {
				hop.RTTMs = &v
			}
		}
		hop.TimedOut = hop.Address == ""
		out = append(out, hop)
	}
	return out
}

func splitLines(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		if r != '\r' {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
