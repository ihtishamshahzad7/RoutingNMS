package alerts

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/incidents"
)

// Evaluator is the Sprint 2 background loop that wires the previously dormant
// in-memory alerts Engine to persisted rules (alert_rules) and real metric
// history. Every ALERT_EVAL_INTERVAL_SECONDS (default 60s) it loads enabled
// evaluable rules, checks the latest per-device metric samples against each
// rule's condition, honors the rule's for-duration breach window, and on a new
// breach fires an incident through the IncidentBridge (durable ai_incidents +
// live in-memory incident + SSE stream) and the Notifier (channel fanout).
//
// Before this loop, nothing in the codebase ever fed the incident engine or
// consulted alert_rules: this is the connection point Sprint 2 exists to add.
type Evaluator struct {
	Engine   *Engine
	Repo     Repository
	Bridge   IncidentBridge
	Notifier Notifier
	Interval time.Duration
	Lookback time.Duration

	mu          sync.Mutex
	lastRun     time.Time
	lastEval    int
	lastFired   int
	running     bool
	breachSince map[string]time.Time // "ruleID:deviceID" -> first breach seen
}

// NewEvaluator wires a fresh in-memory Engine. interval <= 0 defaults to 60s.
func NewEvaluator(repo Repository, live *incidents.Engine, stream *incidents.Stream) *Evaluator {
	return &Evaluator{
		Engine:      NewEngine(),
		Repo:        repo,
		Bridge:      IncidentBridge{Live: live, Stream: stream, Repo: repo, Analyzer: &RCA{}},
		Notifier:    Notifier{Repo: repo},
		Interval:    60 * time.Second,
		Lookback:    3 * time.Minute,
		breachSince: map[string]time.Time{},
	}
}

// Run starts the periodic evaluation loop, mirroring ping.Poller.Run:
// evaluate immediately, then every Interval.
func (e *Evaluator) Run(ctx context.Context) {
	ticker := time.NewTicker(e.ckInterval())
	defer ticker.Stop()
	e.evaluateOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluateOnce(ctx)
		}
	}
}

func (e *Evaluator) ckInterval() time.Duration {
	if e.Interval <= 0 {
		return 60 * time.Second
	}
	return e.Interval
}

// EvaluateNow runs a single evaluation synchronously (usable by a manual
// "evaluate now" endpoint and for tests).
func (e *Evaluator) EvaluateNow(ctx context.Context) (fired int) {
	return e.evaluateOnce(ctx)
}

func (e *Evaluator) evaluateOnce(ctx context.Context) int {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return e.lastFired
	}
	e.running = true
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	started := time.Now().UTC()
	rules, err := e.Repo.ListRules(ctx)
	if err != nil {
		log.Printf("alerts evaluator: list rules: %v", err)
		return 0
	}

	deviceIDs, err := e.deviceIDs(ctx)
	if err != nil {
		log.Printf("alerts evaluator: device ids: %v", err)
		return 0
	}

	evaluated, fired := 0, 0
	for _, pr := range rules {
		rule, ok := toRule(pr)
		if !ok {
			continue
		}
		evaluateImmediate(e, ctx, pr, rule, deviceIDs, &evaluated, &fired)
	}

	e.mu.Lock()
	e.lastRun = started
	e.lastEval = evaluated
	e.lastFired = fired
	e.mu.Unlock()
	return fired
}

// evaluateImmediate checks the latest sample of each device against the rule
// and fires/recover alerts as appropriate. It evaluates threshold/icmp rules;
// absence and traps rules are not sample-driven and are skipped by toRule.
func evaluateImmediate(e *Evaluator, ctx context.Context, pr PersistedRule, rule Rule, deviceIDs []string, evaluated, fired *int) {
	now := time.Now().UTC()
	latest := e.latestSamples(ctx, "device", rule.Metric)
	key := strconv.FormatInt(pr.ID, 10) + ":" + rule.Metric

	// Evaluate every device we know for this metric.
	for _, deviceID := range deviceIDs {
		latestSample, ok := latest[deviceID]
		breachKey := key + ":" + deviceID
		if !ok {
			// No sample: nothing to evaluate; also clear any pending breach so
			// a gap in metrics does not fire behind the scenes.
			e.clearBreach(breachKey)
			continue
		}
		// The in-memory engine wants a full Sample with the metric/device
		// identity; latestSamples only returns the numeric value+time.
		engineSample := Sample{
			Metric:    rule.Metric,
			DeviceID:  deviceID,
			Value:     latestSample.Value,
			Timestamp: latestSample.Timestamp,
		}
		*evaluated++

		matched := matches(rule.Operator, engineSample.Value, rule.Threshold)
		if !matched {
			// Condition cleared: recover the alert if it was active.
			e.clearBreach(breachKey)
			if _, recovered := e.Engine.Recover(rule, engineSample); recovered {
				log.Printf("alerts evaluator: recovered rule %s on device %s", rule.Key, deviceID)
			}
			continue
		}

		// Breach in progress: honor for-duration before firing.
		if rule.For > 0 {
			first, seen := e.firstBreach(breachKey, now)
			if !seen || now.Sub(first) < rule.For {
				continue // not sustained long enough yet
			}
		}
		alert, isNew := e.Engine.Evaluate(rule, engineSample)
		if !isNew {
			continue // already fired; don't re-open an incident
		}
		*fired++
		e.fire(ctx, *alert, pr)
	}
}

// fire opens a durable incident + notifies the rule's channels.
func (e *Evaluator) fire(ctx context.Context, alert Alert, pr PersistedRule) {
	durable, err := e.Bridge.Open(ctx, alert)
	if err != nil {
		log.Printf("alerts evaluator: open incident: %v", err)
	}
	title := pr.Name + " breached"
	if title == " breached" {
		title = "Alert " + pr.RuleType + " breached"
	}
	body := "Rule " + pr.Name + " (" + pr.RuleType + "): device " + alert.DeviceID +
		" value " + strconv.FormatFloat(alert.Value, 'f', 2, 64) +
		" vs threshold " + strconv.FormatFloat(alert.Threshold, 'f', 2, 64)
	if durable.ID > 0 {
		body += " (incident #" + strconv.FormatInt(durable.ID, 10) + ")"
	}
	e.Notifier.Notify(ctx, pr.NotificationChannelIDs, title, body, string(alert.Severity))
}

// latestSamples returns the most recent sample per subject id for one metric
// within the lookback window. DISTINCT ON keeps the newest row per subject.
func (e *Evaluator) latestSamples(ctx context.Context, subjectType, metric string) map[string]alertsSample {
	out := map[string]alertsSample{}
	if e.Repo.DB == nil {
		return out
	}
	cutoff := time.Now().UTC().Add(-e.ckLookback())
	rows, err := e.Repo.DB.Query(ctx, `SELECT DISTINCT ON (subject_id) subject_id, value
		FROM metric_samples
		WHERE subject_type=$1 AND metric_name=$2 AND recorded_at >= $3
		ORDER BY subject_id, recorded_at DESC`, subjectType, metric, cutoff)
	if err != nil {
		log.Printf("alerts evaluator: latest %s: %v", metric, err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var value float64
		if err := rows.Scan(&id, &value); err == nil {
			out[id] = alertsSample{Value: value, Timestamp: time.Now().UTC()}
		}
	}
	return out
}

func (e *Evaluator) ckLookback() time.Duration {
	if e.Lookback <= 0 {
		return 3 * time.Minute
	}
	return e.Lookback
}

// deviceIDs returns every enabled device id (as string) to evaluate rules
// against. Includes devices with no recent samples so absence/pending rules
// still see them.
func (e *Evaluator) deviceIDs(ctx context.Context) ([]string, error) {
	if e.Repo.DB == nil {
		return nil, nil
	}
	rows, err := e.Repo.DB.Query(ctx, `SELECT id FROM devices WHERE enabled=true ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	var id int64
	for rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out, rows.Err()
}

// Status is the evaluator's last-cycle outcome, for a status endpoint.
func (e *Evaluator) Status() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return map[string]any{
		"lastRun":  e.lastRun,
		"evaluated": e.lastEval,
		"fired":    e.lastFired,
		"running":  e.running,
	}
}

func (e *Evaluator) firstBreach(key string, now time.Time) (time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	first, ok := e.breachSince[key]
	if !ok {
		e.breachSince[key] = now
		return now, false
	}
	return first, true
}

func (e *Evaluator) clearBreach(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.breachSince, key)
}

// alertsSample is the minimal shape the evaluator needs from a metric row.
type alertsSample struct {
	Value     float64
	Timestamp time.Time
}