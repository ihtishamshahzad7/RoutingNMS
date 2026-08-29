package monitoring

import "math"

type HealthInput struct {
    Reachable bool
    LatencyMs float64
    PacketLossPct float64
    SNMPUp bool
}

// Score converts basic transport/management signals into a simple 0-100
// health score. It is intentionally deterministic so the UI can explain it.
func Score(in HealthInput) int {
    if !in.Reachable { return 0 }
    score := 100.0
    score -= math.Min(60, in.PacketLossPct*2)
    switch { case in.LatencyMs > 500: score -= 30; case in.LatencyMs > 200: score -= 15; case in.LatencyMs > 100: score -= 7 }
    if !in.SNMPUp { score -= 10 }
    if score < 0 { score = 0 }; if score > 100 { score = 100 }
    return int(math.Round(score))
}
