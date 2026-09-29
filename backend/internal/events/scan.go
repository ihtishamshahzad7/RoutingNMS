package events

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// scanner sources device down/up and interface down/up events from
// existing, already-populated tables (metric_samples and
// interface_transitions) rather than a new poller -- per the item 3.3a
// instruction not to build a parallel event system. Both watermarks are
// in-memory and start at "now"/"current max id" when the process starts,
// so a restart doesn't replay old history as fresh events; a transition
// that happened while the process was down is simply not backfilled. That
// is the one gap of this approach and is called out as an assumption in
// the 3.3a report.
type scanner struct {
	db   *pgxpool.Pool
	repo Repository

	lastDeviceScan  time.Time
	lastIfTransID   int64
	// Item 3.5: separate watermark per host-metric (cpu_percent,
	// memory_percent) -- each is scanned independently via scanHostMetric,
	// same "in-memory watermark, starts at now() on process start" caveat
	// as lastDeviceScan above.
	lastHostMetricScan map[string]time.Time
	watermarksReady    bool
}

// ScanPeriodically runs both the device-transition and interface-transition
// scans on interval until ctx is cancelled.
func ScanPeriodically(ctx context.Context, db *pgxpool.Pool, repo Repository, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	s := &scanner{db: db, repo: repo}
	s.initWatermarks(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanDevices(ctx)
			s.scanInterfaces(ctx)
			s.scanHostMetric(ctx, "cpu_percent", "cpu_alert_threshold_pct", "CPU", EventHighCPU, EventCPUNormal)
			s.scanHostMetric(ctx, "memory_percent", "memory_alert_threshold_pct", "memory", EventHighMemory, EventMemoryNormal)
		}
	}
}

func (s *scanner) initWatermarks(ctx context.Context) {
	s.lastDeviceScan = time.Now().UTC()
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(MAX(id), 0) FROM interface_transitions`).Scan(&s.lastIfTransID)
	s.lastHostMetricScan = map[string]time.Time{
		"cpu_percent":    time.Now().UTC(),
		"memory_percent": time.Now().UTC(),
	}
	s.watermarksReady = true
}

// scanDevices detects device up<->down transitions in metric_samples'
// "up" metric (the same source and LAG() technique internal/alertsfeed
// uses for its live feed) that happened after the last scan, and records
// one event per transition. The base CTE looks back an extra hour beyond
// the watermark purely so LAG() has the correct "previous value" for a row
// right at the cutoff; the outer WHERE still only emits transitions newer
// than the watermark.
func (s *scanner) scanDevices(ctx context.Context) {
	tickStart := time.Now().UTC()
	rows, err := s.db.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'up'
			  AND recorded_at > $1 - INTERVAL '1 hour'
		)
		SELECT r.subject_id, r.value, r.recorded_at, d.name
		FROM ranked r
		JOIN devices d ON d.id::text = r.subject_id
		WHERE r.recorded_at > $1
		  AND r.prev_value IS NOT NULL AND r.prev_value <> r.value
		  AND d.enabled = true
		ORDER BY r.recorded_at`, s.lastDeviceScan)
	if err != nil {
		log.Printf("events: device transition scan failed: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var subjectID, name string
		var value float64
		var recordedAt time.Time
		if err := rows.Scan(&subjectID, &value, &recordedAt, &name); err != nil {
			log.Printf("events: failed to scan device transition row: %v", err)
			continue
		}
		deviceID, err := parseSubjectID(subjectID)
		if err != nil {
			continue
		}
		eventType, severity, msg := EventDeviceUp, SeverityInfo, fmt.Sprintf("%s is back up", name)
		if value == 0 {
			eventType, severity, msg = EventDeviceDown, SeverityCritical, fmt.Sprintf("%s is down", name)
		}
		if err := s.repo.Record(ctx, NewEvent{
			DeviceID:   &deviceID,
			EventType:  eventType,
			Severity:   severity,
			Message:    msg,
			Source:     SourcePoller,
			RefTable:   "metric_samples",
			RefID:      subjectID,
			OccurredAt: recordedAt,
		}); err != nil {
			log.Printf("events: failed to record device event: %v", err)
		}
	}
	s.lastDeviceScan = tickStart
}

// scanInterfaces converts new rows in interface_transitions (populated by
// item 3.1's ifpoll.Poller on every real up/down change) into events,
// watermarked by id so each transition is processed exactly once.
func (s *scanner) scanInterfaces(ctx context.Context) {
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.device_id, t.oper_up, t.changed_at, d.name,
			COALESCE(i.name, 'if' || t.if_index::text)
		FROM interface_transitions t
		JOIN devices d ON d.id = t.device_id
		LEFT JOIN interfaces i ON i.device_id = t.device_id AND i.if_index = t.if_index
		WHERE t.id > $1
		ORDER BY t.id`, s.lastIfTransID)
	if err != nil {
		log.Printf("events: interface transition scan failed: %v", err)
		return
	}
	defer rows.Close()

	var maxID int64
	for rows.Next() {
		var id, deviceID int64
		var operUp bool
		var changedAt time.Time
		var deviceName, ifName string
		if err := rows.Scan(&id, &deviceID, &operUp, &changedAt, &deviceName, &ifName); err != nil {
			log.Printf("events: failed to scan interface transition row: %v", err)
			continue
		}
		if id > maxID {
			maxID = id
		}
		eventType, severity, msg := EventInterfaceUp, SeverityInfo, fmt.Sprintf("%s: interface %s is up", deviceName, ifName)
		if !operUp {
			eventType, severity, msg = EventInterfaceDown, SeverityWarning, fmt.Sprintf("%s: interface %s is down", deviceName, ifName)
		}
		if err := s.repo.Record(ctx, NewEvent{
			DeviceID:   &deviceID,
			EventType:  eventType,
			Severity:   severity,
			Message:    msg,
			Source:     SourcePoller,
			RefTable:   "interface_transitions",
			RefID:      fmt.Sprintf("%d", id),
			OccurredAt: changedAt,
		}); err != nil {
			log.Printf("events: failed to record interface event: %v", err)
		}
	}
	if maxID > s.lastIfTransID {
		s.lastIfTransID = maxID
	}
}

func parseSubjectID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}

// scanHostMetric detects a device's metric_samples reading (cpu_percent or
// memory_percent, written by internal/hostmetrics' poller -- item 3.5)
// crossing the device's own alert threshold column, and records a
// high_*/*_normal event on each crossing -- level-triggered polling turned
// into edge-triggered events via LAG(), the same technique scanDevices uses
// above for the "up" metric, rather than firing (and letting Record's
// dedup collapse) one event per poll cycle for the whole time a device
// stays over threshold.
//
// thresholdColumn is always one of the two fixed column names this
// function is called with from ScanPeriodically -- a compile-time
// constant, not user input -- so building it into the query string here is
// safe.
func (s *scanner) scanHostMetric(ctx context.Context, metricName, thresholdColumn, label, highType, normalType string) {
	tickStart := time.Now().UTC()
	lastScan := s.lastHostMetricScan[metricName]
	if lastScan.IsZero() {
		lastScan = tickStart
	}
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		WITH ranked AS (
			SELECT m.subject_id, m.recorded_at, d.name,
				(m.value >= d.%s) AS above,
				LAG(m.value >= d.%s) OVER (PARTITION BY m.subject_id ORDER BY m.recorded_at) AS prev_above
			FROM metric_samples m
			JOIN devices d ON d.id::text = m.subject_id
			WHERE m.subject_type = 'device' AND m.metric_name = $1
			  AND m.recorded_at > $2 - INTERVAL '1 hour'
			  AND d.enabled = true
		)
		SELECT subject_id, name, recorded_at, above
		FROM ranked
		WHERE recorded_at > $2 AND prev_above IS NOT NULL AND prev_above <> above
		ORDER BY recorded_at`, thresholdColumn, thresholdColumn), metricName, lastScan)
	if err != nil {
		log.Printf("events: host metric scan (%s) failed: %v", metricName, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var subjectID, name string
		var recordedAt time.Time
		var above bool
		if err := rows.Scan(&subjectID, &name, &recordedAt, &above); err != nil {
			log.Printf("events: failed to scan host metric row (%s): %v", metricName, err)
			continue
		}
		deviceID, err := parseSubjectID(subjectID)
		if err != nil {
			continue
		}
		eventType, severity, msg := normalType, SeverityInfo, fmt.Sprintf("%s: %s usage is back below the alert threshold", name, label)
		if above {
			eventType, severity, msg = highType, SeverityWarning, fmt.Sprintf("%s: %s usage is above the alert threshold", name, label)
		}
		if err := s.repo.Record(ctx, NewEvent{
			DeviceID:   &deviceID,
			EventType:  eventType,
			Severity:   severity,
			Message:    msg,
			Source:     SourcePoller,
			RefTable:   "metric_samples",
			RefID:      subjectID,
			OccurredAt: recordedAt,
		}); err != nil {
			log.Printf("events: failed to record host metric event (%s): %v", metricName, err)
		}
	}
	s.lastHostMetricScan[metricName] = tickStart
}
