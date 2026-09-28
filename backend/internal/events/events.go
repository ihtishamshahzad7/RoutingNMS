// Package events implements item 3.3a of the SNMP & Syslog Monitoring
// build: a single, deduplicated event log unifying device down/up
// (internal/devices' metric_samples "up" transitions), interface down/up
// (internal/ifpoll's interface_transitions), and classified syslog
// messages (internal/syslog). Every other source -- SNMP-trap-derived
// events, and item 3.5's future CPU/memory events -- plugs into the same
// model by calling Repository.Record with a new EventType; no schema
// change is needed for that.
package events

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Source values for the events.source column.
const (
	SourceSNMP   = "snmp"
	SourceSyslog = "syslog"
	SourcePoller = "poller"
)

// Severity values for the events.severity column.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Event-type vocabulary shared across sources -- a port flap reported via
// syslog and one detected by SNMP polling both use "interface_down", so the
// unified feed shows one kind of event regardless of how it was detected.
const (
	EventDeviceDown         = "device_down"
	EventDeviceUp           = "device_up"
	EventInterfaceDown      = "interface_down"
	EventInterfaceUp        = "interface_up"
	EventLoopDetected       = "loop_detected"
	EventMACFlapping        = "mac_flapping"
	EventSTPTopologyChange  = "stp_topology_change"
	EventAuthFailure        = "auth_failure"
	EventDeviceReboot       = "device_reboot"
	EventSyslog             = "syslog" // unmatched syslog: no classifier pattern fired
)

// DefaultDedupWindow is used when Repository.DedupWindow is unset.
const DefaultDedupWindow = 60 * time.Second

// Repository is the events store. DedupWindow controls how long an
// identical repeated event from the same device collapses into a single
// row (bumping count/last_seen) instead of inserting a new one; it
// defaults to DefaultDedupWindow (60s, matching the item 3.3a spec) when
// zero, and is set from EVENTS_DEDUP_WINDOW_SECONDS in cmd/api/main.go.
type Repository struct {
	DB         *pgxpool.Pool
	DedupWindow time.Duration
}

func (r Repository) window() time.Duration {
	if r.DedupWindow > 0 {
		return r.DedupWindow
	}
	return DefaultDedupWindow
}

// NewEvent is the input to Record. DeviceID nil means the event isn't
// scoped to a single device (reserved for future sources; every source
// implemented in 3.3a sets it). OccurredAt is when the underlying
// condition was observed (the sample's recorded_at, the transition's
// changed_at, or the syslog message's timestamp) -- not necessarily
// time.Now(), since events are usually recorded slightly after the fact by
// a periodic scan.
type NewEvent struct {
	DeviceID   *int64
	EventType  string
	Severity   string
	Message    string
	Source     string
	RefTable   string
	RefID      string
	OccurredAt time.Time
}

// Record inserts a new event, or -- if an event with the same dedup key
// (device + event type + message) was last seen within the dedup window --
// bumps that existing row's count and last_seen instead. This is what
// keeps a flapping port from burying the event list in repeats.
func (r Repository) Record(ctx context.Context, ev NewEvent) error {
	if r.DB == nil {
		return fmt.Errorf("events repository is not initialized")
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}
	dedupKey := dedupKey(ev.DeviceID, ev.EventType, ev.Message)
	cutoff := ev.OccurredAt.Add(-r.window())

	// Try to fold into the most recent matching row first. last_seen can
	// move backward or forward relative to what's stored (scans process
	// events roughly in order but not strictly), so GREATEST/LEAST keep
	// first_seen/last_seen accurate either way.
	tag, err := r.DB.Exec(ctx, `
		UPDATE events SET
			count = count + 1,
			first_seen = LEAST(first_seen, $2),
			last_seen = GREATEST(last_seen, $2),
			updated_at = NOW()
		WHERE dedup_key = $1
		  AND last_seen >= $3
		  AND id = (
			SELECT id FROM events WHERE dedup_key = $1 AND last_seen >= $3
			ORDER BY last_seen DESC LIMIT 1
		  )`,
		dedupKey, ev.OccurredAt, cutoff)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	var deviceIDArg any
	if ev.DeviceID != nil {
		deviceIDArg = *ev.DeviceID
	}
	_, err = r.DB.Exec(ctx, `
		INSERT INTO events
			(device_id, group_id, event_type, severity, message, source, ref_table, ref_id,
			 dedup_key, count, first_seen, last_seen)
		VALUES ($1,
			(SELECT group_id FROM device_group_members
				WHERE subject_type = 'device' AND subject_id = $1::text LIMIT 1),
			$2, $3, $4, $5, $6, $7, $8, 1, $9, $9)`,
		deviceIDArg, ev.EventType, ev.Severity, ev.Message, ev.Source, ev.RefTable, ev.RefID,
		dedupKey, ev.OccurredAt)
	return err
}

func dedupKey(deviceID *int64, eventType, message string) string {
	d := "none"
	if deviceID != nil {
		d = fmt.Sprintf("%d", *deviceID)
	}
	sum := sha256.Sum256([]byte(d + "|" + eventType + "|" + message))
	return hex.EncodeToString(sum[:])
}
