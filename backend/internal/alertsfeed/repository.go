// Package alertsfeed aggregates every "something is down/alerting" source
// this NMS already tracks -- open OLT optical alerts, devices whose latest
// health sample shows them unreachable, and recent SNMP traps at
// warning/critical severity -- into one unified feed. It exists purely to
// back the browser voice-alert feature: the frontend polls one endpoint
// instead of stitching together three unrelated APIs itself.
package alertsfeed

import (
	"context"
	"fmt"
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
	SourceHTTP   Source = "http"   // a device's optional HTTP(S)+keyword monitor (ported from Uptime Kuma)
)

// Active is one currently-active thing worth alerting an operator about.
type Active struct {
	ID       string    `json:"id"`
	Source   Source    `json:"source"`
	Severity string    `json:"severity"` // critical|warning|info
	Hostname string    `json:"hostname"` // device/OLT name, or the trap's source IP when no name is known
	Message  string    `json:"message"`
	Since    time.Time `json:"since"`
	// Kind distinguishes a still-ongoing problem ("down", the default,
	// omitted from JSON for backward compatibility with existing clients)
	// from a just-recovered one ("up") -- an OLT alert that cleared, or a
	// device whose latest health sample flipped back to reachable, within
	// RecoveryLookback. The voice-alert feature announces both.
	Kind string `json:"kind,omitempty"`
	// subjectType/subjectID identify the underlying device/OLT for
	// maintenance-window suppression -- not serialized, since existing
	// clients key off ID/Source/Hostname already.
	subjectType string
	subjectID   string
}

// MaintenanceChecker answers "which devices/OLTs are currently under an
// active maintenance window" -- satisfied by maintenance.Checker. Declared
// as an interface here (rather than importing internal/maintenance
// directly) to keep this package's dependency graph shallow; main.go wires
// the concrete implementation in.
type MaintenanceChecker interface {
	ActiveSubjects(ctx context.Context) (map[string]bool, error)
}

type Repository struct {
	DB          *pgxpool.Pool
	Maintenance MaintenanceChecker // optional; nil means no suppression
}

// TrapLookback bounds how far back a trap still counts as "active" --
// unlike OLT alerts (which have an explicit open/cleared lifecycle) and
// device health (which is a live probe), a trap is a one-shot event with
// no natural "still ongoing" state, so recency is used as a proxy.
const TrapLookback = 15 * time.Minute

// RecoveryLookback bounds how long a resolved OLT alert or a device that
// just came back up still counts as "recently recovered" -- like
// TrapLookback, recency is the only signal available since a recovery is a
// one-shot transition, not an ongoing state.
const RecoveryLookback = 15 * time.Minute

// List returns every currently-active alert (still down) plus every
// recently-recovered one (back up), across all three sources, newest first.
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

	oltRecovered, err := r.oltRecovered(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, oltRecovered...)

	deviceAlerts, err := r.downDevices(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, deviceAlerts...)

	deviceRecovered, err := r.recoveredDevices(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, deviceRecovered...)

	trapAlerts, err := r.recentTraps(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, trapAlerts...)

	httpAlerts, err := r.downHTTP(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, httpAlerts...)

	httpRecovered, err := r.recoveredHTTP(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, httpRecovered...)

	certExpiring, err := r.certExpiringSoon(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, certExpiring...)

	return r.suppressMaintenance(ctx, out)
}

// suppressMaintenance drops "down" alerts (device/HTTP/cert/OLT) for any
// subject currently covered by an active maintenance window -- ported from
// Uptime Kuma's maintenance-window feature, whose whole point is that a
// planned truck roll or firmware upgrade doesn't page anyone. Recovery
// ("up") events and traps (which carry no subject) are never suppressed, so
// an operator still hears "it's back" even if the window is still open.
func (r Repository) suppressMaintenance(ctx context.Context, in []Active) ([]Active, error) {
	if r.Maintenance == nil {
		return in, nil
	}
	active, err := r.Maintenance.ActiveSubjects(ctx)
	if err != nil || len(active) == 0 {
		return in, err
	}
	out := make([]Active, 0, len(in))
	for _, a := range in {
		if a.Kind != "up" && a.subjectType != "" && active[a.subjectType+":"+a.subjectID] {
			continue
		}
		out = append(out, a)
	}
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
		a.Kind = "down"
		a.subjectType, a.subjectID = "olt", strconv.FormatInt(id, 10)
		out = append(out, a)
	}
	return out, rows.Err()
}

// oltRecovered finds OLT alerts that cleared within RecoveryLookback -- an
// operator who just heard "critical alert on OLT-3" wants to also hear when
// it clears.
func (r Repository) oltRecovered(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT a.id, a.severity, o.name, a.message, a.cleared_at
		FROM olt_alerts a
		JOIN olts o ON o.id = a.olt_id
		WHERE a.status = 'cleared' AND a.cleared_at >= $1
		ORDER BY a.cleared_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id int64
		var a Active
		if err := rows.Scan(&id, &a.Severity, &a.Hostname, &a.Message, &a.Since); err != nil {
			return nil, err
		}
		a.ID = "olt-recovered-" + strconv.FormatInt(id, 10)
		a.Source = SourceOLT
		a.Kind = "up"
		a.subjectType, a.subjectID = "olt", strconv.FormatInt(id, 10)
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
			ID:          "device-" + id,
			Source:      SourceDevice,
			Severity:    "critical",
			Hostname:    name,
			Message:     "device unreachable (" + address + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredDevices finds devices whose latest "up" metric sample flipped
// back to reachable (value=1) after a most-recent-prior sample that was
// down (value=0), recorded within RecoveryLookback -- i.e. it just came
// back up, not merely "is currently up and always was".
func (r Repository) recoveredDevices(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.address, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true
		ORDER BY l.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
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
			ID:          "device-recovered-" + id,
			Source:      SourceDevice,
			Severity:    "info",
			Hostname:    name,
			Message:     "device back online (" + address + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// downHTTP finds devices whose latest "http_up" metric sample (recorded by
// devices.SamplePeriodically's optional HTTP(S)+keyword check) reported
// down -- independent of the device's SNMP/ICMP "up" metric, since a device
// can be pingable but serving a broken/wrong-status web UI.
func (r Repository) downHTTP(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'http_up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.http_url, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 0 AND d.enabled = true AND d.http_check_enabled = true
		ORDER BY l.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name, httpURL string
		var since time.Time
		if err := rows.Scan(&id, &name, &httpURL, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "http-" + id,
			Source:      SourceHTTP,
			Severity:    "warning",
			Hostname:    name,
			Message:     "HTTP check failing (" + httpURL + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredHTTP is downHTTP's recovery counterpart, mirroring
// recoveredDevices for the http_up metric.
func (r Repository) recoveredHTTP(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'http_up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.http_url, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true AND d.http_check_enabled = true
		ORDER BY l.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name, httpURL string
		var since time.Time
		if err := rows.Scan(&id, &name, &httpURL, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "http-recovered-" + id,
			Source:      SourceHTTP,
			Severity:    "info",
			Hostname:    name,
			Message:     "HTTP check recovered (" + httpURL + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// CertExpiryWarningDays is the threshold (ported from Uptime Kuma's default
// certificate-expiry notification) below which a TLS cert still counts as
// "expiring soon".
const CertExpiryWarningDays = 14

// certExpiringSoon finds HTTPS-monitored devices whose latest observed
// certificate expiry is within CertExpiryWarningDays. Unlike the down/up
// pairs above this has no natural "since" transition -- it's reported for
// as long as the cert stays within the window, at reduced severity based on
// how close expiry actually is.
func (r Repository) certExpiringSoon(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'http_cert_expiry_days'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.http_url, l.value, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value <= $1 AND d.enabled = true AND d.http_check_enabled = true
		ORDER BY l.value ASC`, float64(CertExpiryWarningDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name, httpURL string
		var daysLeft float64
		var since time.Time
		if err := rows.Scan(&id, &name, &httpURL, &daysLeft, &since); err != nil {
			return nil, err
		}
		severity := "warning"
		if daysLeft <= 3 {
			severity = "critical"
		}
		out = append(out, Active{
			ID:          "cert-expiry-" + id,
			Source:      SourceHTTP,
			Severity:    severity,
			Hostname:    name,
			Message:     fmt.Sprintf("TLS certificate for %s expires in %.0f day(s)", httpURL, daysLeft),
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
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
		a.Kind = "down"
		a.Message = "SNMP trap " + trapOID
		out = append(out, a)
	}
	return out, rows.Err()
}
