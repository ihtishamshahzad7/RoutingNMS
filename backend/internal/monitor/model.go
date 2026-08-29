package monitor

import "time"

// Device represents a network endpoint monitored by RoutingNMS.
type Device struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Address         string        `json:"address"`
	Type            string        `json:"type"`
	Vendor          string        `json:"vendor,omitempty"`
	SNMPEnabled     bool          `json:"snmpEnabled"`
	SNMPVersion     string        `json:"snmpVersion,omitempty"`
	PollingInterval time.Duration `json:"pollingIntervalSeconds"`
	Enabled         bool          `json:"enabled"`
}

type ProbeResult struct {
	DeviceID  string        `json:"deviceId"`
	Address   string        `json:"address"`
	Reachable bool          `json:"reachable"`
	Latency   time.Duration `json:"latencyNs"`
	CheckedAt time.Time     `json:"checkedAt"`
	Error     string        `json:"error,omitempty"`
}
