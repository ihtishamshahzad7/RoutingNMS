package monitor

import "time"

// Metric is the transport-neutral representation used by pollers before
// persistence in a time-series backend such as VictoriaMetrics.
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func ProbeMetrics(result ProbeResult) []Metric {
	labels := map[string]string{"device_id": result.DeviceID, "address": result.Address}
	latencyMs := float64(result.Latency) / float64(time.Millisecond)
	return []Metric{
		{Name: "device_reachable", Value: boolFloat(result.Reachable), Timestamp: result.CheckedAt, Labels: labels},
		{Name: "device_latency_ms", Value: latencyMs, Timestamp: result.CheckedAt, Labels: labels},
	}
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
