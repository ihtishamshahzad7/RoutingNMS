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
	SourceOLT          Source = "olt"           // an open optical/hardware alert on an OLT/PON/ONU
	SourceDevice       Source = "device"        // a monitored device whose latest health probe is unreachable
	SourceTrap         Source = "trap"          // a recent SNMP trap matched to warning/critical by a trap rule
	SourceHTTP         Source = "http"          // a device's optional HTTP(S)+keyword monitor (ported from Uptime Kuma)
	SourceICMP         Source = "icmp"          // a device's dedicated ICMP ping poller (internal/ping), independent of the TCP/SNMP "up" check
	SourceDNS          Source = "dns"           // a device's optional DNS resolution monitor (ported from Uptime Kuma's "DNS" monitor type)
	SourcePush         Source = "push"          // a device's optional "push" heartbeat monitor -- no push arrived within interval+grace
	SourceSSH          Source = "ssh"           // a device's optional SSH reachability monitor (TCP-connect + optional banner match)
	SourceTelnet       Source = "telnet"        // a device's optional Telnet reachability monitor, mirroring SourceSSH
	SourceTopologyLink Source = "topology_link" // a manually-mapped device-to-device port link whose SNMP ifOperStatus reports down on either end
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

	icmpAlerts, err := r.downICMP(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, icmpAlerts...)

	icmpRecovered, err := r.recoveredICMP(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, icmpRecovered...)

	dnsAlerts, err := r.downDNS(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, dnsAlerts...)

	dnsRecovered, err := r.recoveredDNS(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, dnsRecovered...)

	pushAlerts, err := r.downPush(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, pushAlerts...)

	pushRecovered, err := r.recoveredPush(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, pushRecovered...)

	sshAlerts, err := r.downSSH(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, sshAlerts...)

	sshRecovered, err := r.recoveredSSH(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, sshRecovered...)

	telnetAlerts, err := r.downTelnet(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, telnetAlerts...)

	telnetRecovered, err := r.recoveredTelnet(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, telnetRecovered...)

	topoLinkAlerts, err := r.downTopologyLinks(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, topoLinkAlerts...)

	topoLinkRecovered, err := r.recoveredTopologyLinks(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, topoLinkRecovered...)

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

// downICMP finds devices whose latest "icmp_reachable" metric sample
// (recorded by the dedicated ICMP poller in internal/ping, distinct from the
// TCP/SNMP-based "up" check) reported unreachable -- a device can answer TCP
// or SNMP but stop responding to ICMP (a firewall change, an overloaded CPU
// deprioritizing ping), so this is tracked as its own alert source.
func (r Repository) downICMP(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'icmp_reachable'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.address, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 0 AND d.enabled = true AND d.icmp_enabled = true
		ORDER BY l.recorded_at DESC`)
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
			ID:          "icmp-" + id,
			Source:      SourceICMP,
			Severity:    "warning",
			Hostname:    name,
			Message:     "ICMP ping failing (" + address + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredICMP is downICMP's recovery counterpart, mirroring
// recoveredDevices/recoveredHTTP for the icmp_reachable metric.
func (r Repository) recoveredICMP(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'icmp_reachable'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.address, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true AND d.icmp_enabled = true
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
			ID:          "icmp-recovered-" + id,
			Source:      SourceICMP,
			Severity:    "info",
			Hostname:    name,
			Message:     "ICMP ping recovered (" + address + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// downDNS finds devices whose latest "dns_up" metric sample (recorded by
// the DNS resolution poller, internal/dnscheck) reported down.
func (r Repository) downDNS(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'dns_up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.dns_hostname, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 0 AND d.enabled = true AND d.dns_enabled = true
		ORDER BY l.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name, hostname string
		var since time.Time
		if err := rows.Scan(&id, &name, &hostname, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "dns-" + id,
			Source:      SourceDNS,
			Severity:    "warning",
			Hostname:    name,
			Message:     "DNS resolution failing (" + hostname + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredDNS is downDNS's recovery counterpart, mirroring
// recoveredHTTP/recoveredICMP for the dns_up metric.
func (r Repository) recoveredDNS(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'dns_up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, d.dns_hostname, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true AND d.dns_enabled = true
		ORDER BY l.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name, hostname string
		var since time.Time
		if err := rows.Scan(&id, &name, &hostname, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "dns-recovered-" + id,
			Source:      SourceDNS,
			Severity:    "info",
			Hostname:    name,
			Message:     "DNS resolution recovered (" + hostname + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// downPush finds devices whose latest "push_up" metric sample (recorded by
// the push heartbeat sweeper, internal/push) reported down -- no push
// arrived within push_interval_seconds + push_grace_period_seconds.
func (r Repository) downPush(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'push_up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 0 AND d.enabled = true AND d.push_enabled = true
		ORDER BY l.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name string
		var since time.Time
		if err := rows.Scan(&id, &name, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "push-" + id,
			Source:      SourcePush,
			Severity:    "critical",
			Hostname:    name,
			Message:     "no heartbeat push received in time",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredPush is downPush's recovery counterpart.
func (r Repository) recoveredPush(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'push_up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true AND d.push_enabled = true
		ORDER BY l.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name string
		var since time.Time
		if err := rows.Scan(&id, &name, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "push-recovered-" + id,
			Source:      SourcePush,
			Severity:    "info",
			Hostname:    name,
			Message:     "heartbeat push recovered",
			Since:       since,
			Kind:        "up",
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

// downSSH finds devices whose latest "ssh_up" metric sample (recorded by
// the SSH reachability poller, internal/sshcheck) reported down.
func (r Repository) downSSH(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'ssh_up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 0 AND d.enabled = true AND d.ssh_enabled = true
		ORDER BY l.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name string
		var since time.Time
		if err := rows.Scan(&id, &name, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "ssh-" + id,
			Source:      SourceSSH,
			Severity:    "warning",
			Hostname:    name,
			Message:     "SSH unreachable (" + name + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredSSH is downSSH's recovery counterpart.
func (r Repository) recoveredSSH(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'ssh_up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true AND d.ssh_enabled = true
		ORDER BY l.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name string
		var since time.Time
		if err := rows.Scan(&id, &name, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "ssh-recovered-" + id,
			Source:      SourceSSH,
			Severity:    "info",
			Hostname:    name,
			Message:     "SSH recovered (" + name + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// downTelnet finds devices whose latest "telnet_up" metric sample (recorded
// by the Telnet reachability poller, internal/telnetcheck) reported down.
func (r Repository) downTelnet(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'telnet_up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 0 AND d.enabled = true AND d.telnet_enabled = true
		ORDER BY l.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name string
		var since time.Time
		if err := rows.Scan(&id, &name, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "telnet-" + id,
			Source:      SourceTelnet,
			Severity:    "warning",
			Hostname:    name,
			Message:     "Telnet unreachable (" + name + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredTelnet is downTelnet's recovery counterpart.
func (r Repository) recoveredTelnet(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'device' AND metric_name = 'telnet_up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT d.id, d.name, l.recorded_at
		FROM latest l
		JOIN devices d ON d.id::text = l.subject_id
		WHERE l.value = 1 AND l.prev_value = 0 AND l.recorded_at >= $1 AND d.enabled = true AND d.telnet_enabled = true
		ORDER BY l.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, name string
		var since time.Time
		if err := rows.Scan(&id, &name, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "telnet-recovered-" + id,
			Source:      SourceTelnet,
			Severity:    "info",
			Hostname:    name,
			Message:     "Telnet recovered (" + name + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "device",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// downTopologyLinks finds topology links (internal/topolinks) whose latest
// "port_up" metric sample reported down -- either end's SNMP ifOperStatus
// is not up. Message names the group/device/port explicitly, e.g.
// "group Core Ring device Switch-1 port eth1 down".
func (r Repository) downTopologyLinks(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, recorded_at
			FROM metric_samples
			WHERE subject_type = 'topology_link' AND metric_name = 'port_up'
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT tl.id, g.name, da.name, tl.interface_a, db.name, tl.interface_b, lat.recorded_at
		FROM latest lat
		JOIN topo_links tl ON tl.id::text = lat.subject_id
		JOIN topo_link_groups g ON g.id = tl.group_id
		JOIN devices da ON da.id = tl.device_a_id
		JOIN devices db ON db.id = tl.device_b_id
		WHERE lat.value = 0
		ORDER BY lat.recorded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, group, nameA, ifaceA, nameB, ifaceB string
		var since time.Time
		if err := rows.Scan(&id, &group, &nameA, &ifaceA, &nameB, &ifaceB, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "topology-link-" + id,
			Source:      SourceTopologyLink,
			Severity:    "critical",
			Hostname:    nameA + " <-> " + nameB,
			Message:     "group " + group + " device " + nameA + " port " + ifaceA + " down (link to " + nameB + " port " + ifaceB + ")",
			Since:       since,
			Kind:        "down",
			subjectType: "topology_link",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}

// recoveredTopologyLinks is downTopologyLinks's recovery counterpart.
func (r Repository) recoveredTopologyLinks(ctx context.Context) ([]Active, error) {
	rows, err := r.DB.Query(ctx, `
		WITH ranked AS (
			SELECT subject_id, value, recorded_at,
				LAG(value) OVER (PARTITION BY subject_id ORDER BY recorded_at) AS prev_value
			FROM metric_samples
			WHERE subject_type = 'topology_link' AND metric_name = 'port_up'
		),
		latest AS (
			SELECT DISTINCT ON (subject_id) subject_id, value, prev_value, recorded_at
			FROM ranked
			ORDER BY subject_id, recorded_at DESC
		)
		SELECT tl.id, g.name, da.name, tl.interface_a, db.name, tl.interface_b, lat.recorded_at
		FROM latest lat
		JOIN topo_links tl ON tl.id::text = lat.subject_id
		JOIN topo_link_groups g ON g.id = tl.group_id
		JOIN devices da ON da.id = tl.device_a_id
		JOIN devices db ON db.id = tl.device_b_id
		WHERE lat.value = 1 AND lat.prev_value = 0 AND lat.recorded_at >= $1
		ORDER BY lat.recorded_at DESC`, time.Now().Add(-RecoveryLookback))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var id, group, nameA, ifaceA, nameB, ifaceB string
		var since time.Time
		if err := rows.Scan(&id, &group, &nameA, &ifaceA, &nameB, &ifaceB, &since); err != nil {
			return nil, err
		}
		out = append(out, Active{
			ID:          "topology-link-recovered-" + id,
			Source:      SourceTopologyLink,
			Severity:    "info",
			Hostname:    nameA + " <-> " + nameB,
			Message:     "group " + group + " device " + nameA + " port " + ifaceA + " recovered (link to " + nameB + " port " + ifaceB + ")",
			Since:       since,
			Kind:        "up",
			subjectType: "topology_link",
			subjectID:   id,
		})
	}
	return out, rows.Err()
}
