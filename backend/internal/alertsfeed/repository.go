// Package alertsfeed aggregates every "something is down/alerting" source
// this NMS already tracks -- open OLT optical alerts, devices whose latest
// health sample shows them unreachable, and recent SNMP traps at
// warning/critical severity -- into one unified feed. It exists purely to
// back the browser voice-alert feature: the frontend polls one endpoint
// instead of stitching together three unrelated APIs itself.
package alertsfeed

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Source identifies which subsystem an alert came from -- this is the
// "alert type" an operator can filter on in the voice-alert settings.
type Source string

const (
	SourceOLT    Source = "olt"    // an open optical/hardware alert on an OLT/PON/ONU
	SourceDevice Source = "device" // a monitored device whose latest health probe is unreachable
	SourceTrap   Source = "trap"   // a recent SNMP trap matched to warning/critical by a trap rule
)

// Active is one currently-active thing worth alerting an operator about.
type Active struct {
	ID       string    `json:"id"`
	Source   Source    `json:"source"`
	Severity string    `json:"severity"` // critical|warning|info
	Hostname string    `json:"hostname"` // device/OLT name, or the trap's source IP when no name is known
	Message  string    `json:"message"`
	Since    time.Time `json:"since"`
}

type Repository struct{ DB *pgxpool.Pool }

// TrapLookback bounds how far back a trap still counts as "active" --
// unlike OLT alerts (which have an explicit open/cleared lifecycle) and
// device health (which is a live probe), a trap is a one-shot event with
// no natural "still ongoing" state, so recency is used as a proxy.
const TrapLookback = 15 * time.Minute

// List returns every currently-active alert across all three sources,
// newest first.
func (r Repository) List(ctx context.Context) ([]Active, error) {
	if r.DB == nil {
		return nil, nil
	}
	out := make([]Active, 0, 32)

	oltAlerts, err := r.oltAlerts(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, oltAlerts...)

	deviceAlerts, err := r.downDevices(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, deviceAlerts...)

	trapAlerts, err := r.recentTraps(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, trapAlerts...)

	return out, nil
}

func (r Repository) oltAlerts(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT a.id, a.severity, o.name, a.message, a.last_seen
		FROM olt_alerts a
		JOIN olts o ON o.id = a.olt_id
		WHERE a.status = 'open'
		ORDER BY a.last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var a Active
		var id int64
		if err := rows.Scan(&id, &a.Severity, &a.Hostname, &a.Message, &a.Since); err != nil {
			return nil, err
		}
		a.ID = "olt-" + strconv.FormatInt(id, 10)
		a.Source = SourceOLT
		out = append(out, a)
	}
	return out, rows.Err()
}

// downDevices finds devices whose most recent "up" metric sample (recorded
// by devices.SamplePeriodically) reported unreachable, joined back to the
// devices table for a human name.
func (r Repository) downDevices(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest_up AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.address, lu.recorded_at
		FROM latest_up lu
		JOIN devices d ON d.id::text = lu.subject_id
		WHERE lu.value = 0 AND d.enabled = true
		ORDER BY lu.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name, address string
		var since time.Time
		if err := rows.Scan(&id, &name, &address, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:       "device-" + id,
			Source:   SourceDevice,
			Severity: "critical",
			Hostname: name,
			Message:  "device unreachable (" + address + ")",
			Since:    since,
		})
	}
	return out, rows.Err()
}

func (r Repository) recentTraps(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT id, severity, source_ip, COALESCE(NULLIF(trap_oid,''),'unidentified trap'), received_at
		FROM snmp_traps
		WHERE severity IN ('critical','warning') AND received_at >= $1
		ORDER BY received_at DESC`, time.Now().Add(-TrapLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id int64
		var a Active
		var trapOID string
		if err := rows.Scan(&id, &a.Severity, &a.Hostname, &trapOID, &a.Since); err != nil {
			return nil, err
		}
		a.ID = "trap-" + strconv.FormatInt(id, 10)
		a.Source = SourceTrap
		a.Message = "SNMP trap " + trapOID
		out = append(out, a)
	}
	return out, rows.Err()
}
