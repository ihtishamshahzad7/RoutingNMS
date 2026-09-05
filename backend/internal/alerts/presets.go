package alerts

// Preset is a friendly, one-click alert-rule template surfaced by
// GET /api/v1/alerts/presets. It maps a human-readable name/description to
// the exact {metric, operator, threshold} triple the manual rule-creation
// form otherwise requires a user to type by hand -- so "alert me when this
// device's ping goes down" doesn't require already knowing that the metric
// is named "icmp_reachable".
//
// Presets are defined here (backend), not hardcoded in the frontend, so a
// new monitor type only needs a catalog entry added in one place; the
// frontend just renders whatever this endpoint returns and pre-fills the
// existing rule-creation form untouched. The metric names below are taken
// directly from the pollers that record them (see MetricName literals in
// internal/ping, internal/dnscheck, internal/sshcheck, internal/telnetcheck,
// internal/push and internal/devices).
type Preset struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	RuleType    string  `json:"ruleType"`
	Metric      string  `json:"metric"`
	Operator    string  `json:"operator"`
	Threshold   float64 `json:"threshold"`
	Unit        string  `json:"unit,omitempty"`
	Severity    string  `json:"severity"`
}

// Presets is the static catalog served by the presets endpoint.
var Presets = []Preset{
	{
		ID: "icmp-down", Name: "Device down (ICMP ping)",
		Description: "Fires when a device stops responding to ICMP ping (internal/ping poller).",
		RuleType:    "threshold",
		Metric:      "icmp_reachable", Operator: "<", Threshold: 1,
		Severity: "critical",
	},
	{
		ID: "http-down", Name: "HTTP/HTTPS check down",
		Description: "Fires when a device's HTTP(S) health check stops returning a healthy response.",
		RuleType:    "threshold",
		Metric:      "http_up", Operator: "<", Threshold: 1,
		Severity: "critical",
	},
	{
		ID: "dns-down", Name: "DNS resolution failed",
		Description: "Fires when a device's configured DNS check fails to resolve.",
		RuleType:    "threshold",
		Metric:      "dns_up", Operator: "<", Threshold: 1,
		Severity: "warning",
	},
	{
		ID: "ssh-down", Name: "SSH unreachable",
		Description: "Fires when a device's SSH port check stops succeeding.",
		RuleType:    "threshold",
		Metric:      "ssh_up", Operator: "<", Threshold: 1,
		Severity: "warning",
	},
	{
		ID: "telnet-down", Name: "Telnet unreachable",
		Description: "Fires when a device's Telnet port check stops succeeding.",
		RuleType:    "threshold",
		Metric:      "telnet_up", Operator: "<", Threshold: 1,
		Severity: "warning",
	},
	{
		ID: "push-missed", Name: "Push monitor missed heartbeat",
		Description: "Fires when a push monitor doesn't receive an expected heartbeat in time.",
		RuleType:    "threshold",
		Metric:      "push_up", Operator: "<", Threshold: 1,
		Severity: "warning",
	},
	{
		ID: "tcp-snmp-down", Name: "Device unreachable (SNMP/TCP)",
		Description: "Fires when a device's generic TCP/SNMP reachability check goes down.",
		RuleType:    "threshold",
		Metric:      "up", Operator: "<", Threshold: 1,
		Severity: "critical",
	},
	{
		ID: "cert-expiring", Name: "HTTPS certificate expiring soon",
		Description: "Fires when a device's HTTPS certificate has fewer than 14 days left before expiry.",
		RuleType:    "threshold",
		Metric:      "http_cert_expiry_days", Operator: "<", Threshold: 14,
		Unit: "days", Severity: "warning",
	},
	{
		ID: "icmp-loss-high", Name: "High packet loss (ICMP)",
		Description: "Fires when a device's ICMP packet loss exceeds 30%.",
		RuleType:    "icmp_loss",
		Metric:      "icmp_loss_pct", Operator: ">", Threshold: 30,
		Unit: "%", Severity: "warning",
	},
	{
		ID: "icmp-rtt-high", Name: "High latency (ICMP RTT)",
		Description: "Fires when a device's ICMP round-trip time exceeds 100ms.",
		RuleType:    "icmp_rtt",
		Metric:      "icmp_rtt_ms", Operator: ">", Threshold: 100,
		Unit: "ms", Severity: "warning",
	},
}
