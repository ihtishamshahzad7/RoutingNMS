package ping

import (
	"math"
	"regexp"
	"strconv"
)

// Parsing helpers for `ping` command output. Handles both the classic Unix
//
//	64 bytes from 1.2.3.4: icmp_seq=1 ttl=64 time=0.451 ms
//	--- 1.2.3.4 ping statistics ---
//	3 packets transmitted, 3 received, 0% packet loss
//	rtt min/avg/max/mdev = 0.434/0.478/0.542/0.046 ms
//
// and Windows
//
//	Reply from 1.2.3.4: bytes=32 time<1ms TTL=64
//	Packets: Sent = 3, Received = 3, Lost = 0 (0% loss)
//	Minimum = 0ms, Maximum = 0ms, Average = 0ms
// styles.

var (
	rePingTime    = regexp.MustCompile(`time[=<]([0-9.]+)\s*ms`)
	reLossPct     = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%\s*(?:packet\s+)?loss`)
	reLossWin     = regexp.MustCompile(`Lost\s*=\s*[0-9]+\s*\(([0-9]+(?:\.[0-9]+)?)%`)
	reAvgUnix     = regexp.MustCompile(`rtt min/avg/max/mdev\s*=\s*[0-9.]+/([0-9.]+)/`)
	reAvgWin      = regexp.MustCompile(`Average\s*=\s*([0-9.]+)ms`)
	reTTLUnix     = regexp.MustCompile(`ttl=(\d+)`)
	reTTLWin      = regexp.MustCompile(`TTL=(\d+)`)
)

// parseLoss extracts the packet-loss percentage from ping output.
func parseLoss(text string) float64 {
	if m := reLossPct.FindStringSubmatch(text); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	if m := reLossWin.FindStringSubmatch(text); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	return -1 // unknown
}

// parseAvgRTT returns the rounded average RTT in ms.
func parseAvgRTT(text string) (float64, bool) {
	if m := reAvgUnix.FindStringSubmatch(text); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v, true
		}
	}
	if m := reAvgWin.FindStringSubmatch(text); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v, true
		}
	}
	// Fall back to the most recent per-packet `time=` if no summary line.
	matches := rePingTime.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1][1]
		if v, err := strconv.ParseFloat(last, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}

// parseRTTList returns every per-packet RTT so the caller can compute jitter.
func parseRTTList(text string) ([]float64, bool) {
	matches := rePingTime.FindAllStringSubmatch(text, -1)
	out := make([]float64, 0, len(matches))
	for _, m := range matches {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			out = append(out, v)
		}
	}
	return out, len(out) > 0
}

// computeJitter returns the mean absolute deviation of the RTT samples (ms),
// a common stand-in for ICMP jitter.
func computeJitter(rtts []float64) float64 {
	if len(rtts) == 0 {
		return 0
	}
	var sum float64
	for _, v := range rtts {
		sum += v
	}
	mean := sum / float64(len(rtts))
	var dev float64
	for _, v := range rtts {
		dev += math.Abs(v - mean)
	}
	return dev / float64(len(rtts))
}

// parseTTL returns the last TTL seen in the output.
func parseTTL(text string) (int, bool) {
	if m := reTTLUnix.FindStringSubmatch(text); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			return v, true
		}
	}
	if m := reTTLWin.FindStringSubmatch(text); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			return v, true
		}
	}
	return 0, false
}
