package events

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pattern is a compiled syslog_event_patterns row.
type pattern struct {
	name      string
	re        *regexp.Regexp
	eventType string
	severity  string
}

// patternCache holds the compiled classification patterns, refreshed
// periodically from the DB so edits to syslog_event_patterns (a future
// admin UI, per the item 3.3a spec's "editable later") take effect without
// a restart. Mirrors internal/syslog's deviceCache refresh pattern.
type patternCache struct {
	mu       sync.RWMutex
	patterns []pattern
}

func (c *patternCache) get() []pattern {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.patterns
}

func (c *patternCache) refresh(ctx context.Context, db *pgxpool.Pool) {
	rows, err := db.Query(ctx, `SELECT name, pattern, event_type, severity
		FROM syslog_event_patterns WHERE enabled = true ORDER BY sort_order, id`)
	if err != nil {
		log.Printf("events: failed to load syslog_event_patterns: %v", err)
		return
	}
	defer rows.Close()

	var compiled []pattern
	for rows.Next() {
		var name, raw, eventType, severity string
		if err := rows.Scan(&name, &raw, &eventType, &severity); err != nil {
			log.Printf("events: failed to scan syslog_event_patterns row: %v", err)
			continue
		}
		re, err := regexp.Compile(raw)
		if err != nil {
			log.Printf("events: skipping syslog pattern %q: invalid regexp: %v", name, err)
			continue
		}
		compiled = append(compiled, pattern{name: name, re: re, eventType: eventType, severity: severity})
	}
	c.mu.Lock()
	c.patterns = compiled
	c.mu.Unlock()
}

func (c *patternCache) runPeriodic(ctx context.Context, db *pgxpool.Pool, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	c.refresh(ctx, db)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx, db)
		}
	}
}

// classify returns the first enabled pattern (in sort_order) whose regexp
// matches body, or ok=false if none do -- the caller falls back to a plain
// "syslog" event per the item 3.3a spec ("Unmatched syslog stays as a
// plain 'syslog' event").
func (c *patternCache) classify(body string) (pattern, bool) {
	for _, p := range c.get() {
		if p.re.MatchString(body) {
			return p, true
		}
	}
	return pattern{}, false
}

// severityFromSyslogLevel maps an RFC 5424 severity number (0=emergency..
// 7=debug) to our three-level vocabulary, used only for the unmatched
// "syslog" fallback event -- classified events use the pattern's own
// configured severity instead.
func severityFromSyslogLevel(level *int) string {
	if level == nil {
		return SeverityInfo
	}
	switch {
	case *level <= 3:
		return SeverityCritical
	case *level <= 4:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

// PatternCache is the exported handle main.go creates once and passes to
// both internal/syslog's receiver (for classification) and nothing else --
// its own periodic refresh goroutine is started by NewPatternCache.
type PatternCache struct {
	cache *patternCache
}

// NewPatternCache creates an (initially empty) pattern cache and starts its
// periodic refresh from the DB. Call this once at startup, before
// syslog.ListenAndServe.
func NewPatternCache(ctx context.Context, db *pgxpool.Pool, refreshInterval time.Duration) *PatternCache {
	c := &patternCache{}
	go c.runPeriodic(ctx, db, refreshInterval)
	return &PatternCache{cache: c}
}

// ClassifySyslog matches a just-stored syslog message against the cached
// patterns and records the resulting event (or a plain "syslog" event if
// nothing matched). Called synchronously from internal/syslog's worker
// right after the message is stored, per the item 3.3a design: that
// worker pool is already the single choke point for every incoming
// message, so a second periodic scan isn't needed for this source.
func (r Repository) ClassifySyslog(ctx context.Context, patterns *PatternCache, msgID int64, deviceID *int64, hostname, body string, syslogSeverity *int, occurredAt time.Time) error {
	if patterns == nil {
		return fmt.Errorf("events: nil pattern cache")
	}
	subject := hostname
	if subject == "" {
		subject = "unknown host"
	}

	if p, ok := patterns.cache.classify(body); ok {
		return r.Record(ctx, NewEvent{
			DeviceID:   deviceID,
			EventType:  p.eventType,
			Severity:   p.severity,
			Message:    fmt.Sprintf("%s: %s", subject, body),
			Source:     SourceSyslog,
			RefTable:   "syslog_messages",
			RefID:      fmt.Sprintf("%d", msgID),
			OccurredAt: occurredAt,
		})
	}
	return r.Record(ctx, NewEvent{
		DeviceID:   deviceID,
		EventType:  EventSyslog,
		Severity:   severityFromSyslogLevel(syslogSeverity),
		Message:    fmt.Sprintf("%s: %s", subject, body),
		Source:     SourceSyslog,
		RefTable:   "syslog_messages",
		RefID:      fmt.Sprintf("%d", msgID),
		OccurredAt: occurredAt,
	})
}
