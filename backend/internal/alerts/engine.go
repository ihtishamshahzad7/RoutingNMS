package alerts

import (
	"fmt"
	"sync"
	"time"
)

type Severity string

const (
	Info     Severity = "info"
	Warning  Severity = "warning"
	Critical Severity = "critical"
)

type Rule struct {
	Key       string
	Metric    string
	Operator  string
	Threshold float64
	Severity  Severity
	For       time.Duration
	// ResendInterval: re-notify every N consecutive breaching ticks since the
	// last notification while the breach stays open (0 = notify once only).
	ResendInterval int
	// UpsideDown inverts the operator/threshold comparison's result -- see
	// breaches().
	UpsideDown bool
}

type Sample struct {
	Metric    string
	DeviceID  string
	Value     float64
	Timestamp time.Time
}

type Alert struct {
	Key        string
	RuleKey    string
	DeviceID   string
	Severity   Severity
	Value      float64
	Threshold  float64
	StartedAt  time.Time
	ResolvedAt *time.Time
}

type Engine struct {
	mu     sync.Mutex
	active map[string]Alert
}

func NewEngine() *Engine { return &Engine{active: make(map[string]Alert)} }

func (e *Engine) Evaluate(rule Rule, sample Sample) (*Alert, bool) {
	if rule.Metric != sample.Metric || !breaches(rule, sample.Value) {
		return nil, false
	}
	key := fmt.Sprintf("%s:%s", rule.Key, sample.DeviceID)
	e.mu.Lock()
	defer e.mu.Unlock()
	if existing, ok := e.active[key]; ok {
		return &existing, false
	}
	alert := Alert{Key: key, RuleKey: rule.Key, DeviceID: sample.DeviceID, Severity: rule.Severity, Value: sample.Value, Threshold: rule.Threshold, StartedAt: sample.Timestamp}
	e.active[key] = alert
	return &alert, true
}

func (e *Engine) Recover(rule Rule, sample Sample) (*Alert, bool) {
	key := fmt.Sprintf("%s:%s", rule.Key, sample.DeviceID)
	e.mu.Lock()
	defer e.mu.Unlock()
	alert, ok := e.active[key]
	if !ok {
		return nil, false
	}
	now := sample.Timestamp
	alert.ResolvedAt = &now
	delete(e.active, key)
	return &alert, true
}

func matches(operator string, value, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "=":
		return value == threshold
	default:
		return false
	}
}

// breaches reports whether value breaches rule's operator/threshold
// condition, honoring the rule's UpsideDown flag (Uptime Kuma's "upside down
// mode"): compute the normal comparison first, then invert it if the rule is
// flagged upside down, so e.g. a rule with operator "<" and upside_down=true
// breaches exactly when value is NOT less than threshold.
func breaches(rule Rule, value float64) bool {
	result := matches(rule.Operator, value, rule.Threshold)
	if rule.UpsideDown {
		result = !result
	}
	return result
}
