package alerts

import (
	"strconv"
	"time"
)

// toRule converts a persisted rule into the in-memory Rule the engine
// evaluates, resolving the condition_config JSONB into typed fields. Only
// threshold/icmp_loss/icmp_rtt rules are evaluable against metric samples; a
// non-evaluable type returns ok=false.
func toRule(p PersistedRule) (Rule, bool) {
	if !p.Enabled {
		return Rule{}, false
	}
	var metric, operator string
	var threshold float64
	getString := func(k string) string {
		if v, ok := p.Condition[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	getFloat := func(k string) float64 {
		if v, ok := p.Condition[k]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case string:
				if f, err := strconv.ParseFloat(n, 64); err == nil {
					return f
				}
			}
		}
		if p.RuleType == "icmp_loss" && metric == "icmp_loss_pct" {
			return 30
		}
		if p.RuleType == "icmp_rtt" && metric == "icmp_rtt_ms" {
			return 100
		}
		return 0
	}

	switch p.RuleType {
	case "threshold", "icmp_loss", "icmp_rtt":
		metric = getString("metric")
		operator = getString("operator")
		if operator == "" {
			operator = ">"
		}
		threshold = getFloat("threshold")
		if metric == "" {
			return Rule{}, false
		}
	default:
		// absence / traps cannot be evaluated against plain metric samples by
		// this engine; skip them in the sample-driven loop.
		return Rule{}, false
	}

	return Rule{
		Key:       strconv.FormatInt(p.ID, 10),
		Metric:    metric,
		Operator:  operator,
		Threshold: threshold,
		Severity:  Severity(p.Severity),
		For:       durSeconds(p.ForDurationSec),
	}, true
}

func durSeconds(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

// metricSubjectType maps a metric name to the subject_type used in
// metric_samples so the evaluator queries the right rows.
func metricSubjectType(ruleType string) string {
	return "device"
}

// ruleIsProbeMetric reports whether a rule's metric is produced by the ICMP
// poller (icmp_*) rather than the generic device sampler.
func ruleIsProbeMetric(metric string) bool {
	switch metric {
	case "icmp_loss_pct", "icmp_rtt_ms", "icmp_reachable":
		return true
	}
	return false
}
