package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Target is a saved subnet the background AutoScanner rescans on a
// schedule, persisted in discovery_targets. Distinct from the pre-existing
// in-memory Job (an ad hoc, operator-triggered one-off scan) -- a Target
// is configuration, a Job is a single run.
type Target struct {
	ID             int64            `json:"id"`
	OrganizationID string           `json:"organizationId"`
	CIDR           string           `json:"cidr"`
	SNMP           snmp.Credentials `json:"-"`
	SNMPPort       uint16           `json:"snmpPort"`
	TimeoutMS      int              `json:"timeoutMs"`
	IntervalSecs   int              `json:"intervalSeconds"`
	Enabled        bool             `json:"enabled"`
	LastScannedAt  *time.Time       `json:"lastScannedAt,omitempty"`
	LastScanError  string           `json:"lastScanError,omitempty"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

// Candidate is one host a scheduled Target scan found responsive,
// persisted in discovery_candidates until reviewed.
type Candidate struct {
	ID               int64     `json:"id"`
	TargetID         int64     `json:"targetId"`
	Address          string    `json:"address"`
	SystemName       string    `json:"systemName,omitempty"`
	SysDescr         string    `json:"sysDescr,omitempty"`
	SysObjectID      string    `json:"sysObjectId,omitempty"`
	DeviceType       string    `json:"deviceType"`
	Vendor           string    `json:"vendor,omitempty"`
	Status           string    `json:"status"`
	FirstSeenAt      time.Time `json:"firstSeenAt"`
	LastSeenAt       time.Time `json:"lastSeenAt"`
	ImportedDeviceID *string   `json:"importedDeviceId,omitempty"`
}

// TargetInput is the create/update payload for a Target.
type TargetInput struct {
	OrganizationID string
	CIDR           string
	SNMP           snmp.Credentials
	SNMPPort       uint16
	TimeoutMS      int
	IntervalSecs   int
	Enabled        bool
}

// TargetRepository persists discovery_targets and discovery_candidates.
type TargetRepository struct {
	DB *pgxpool.Pool
}

func (r TargetRepository) CreateTarget(ctx context.Context, in TargetInput) (Target, error) {
	if r.DB == nil {
		return Target{}, fmt.Errorf("discovery target repository is not initialized")
	}
	if in.SNMPPort == 0 {
		in.SNMPPort = 161
	}
	if in.TimeoutMS <= 0 {
		in.TimeoutMS = 1500
	}
	if in.IntervalSecs < 300 {
		in.IntervalSecs = 3600
	}
	var t Target
	err := r.DB.QueryRow(ctx, `
		INSERT INTO discovery_targets
			(organization_id, cidr, snmp_version, snmp_community, snmp_username,
			 snmp_auth_proto, snmp_auth_pass, snmp_priv_proto, snmp_priv_pass,
			 snmp_port, timeout_ms, interval_seconds, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, organization_id, cidr, snmp_port, timeout_ms, interval_seconds,
			enabled, last_scanned_at, last_scan_error, created_at, updated_at`,
		in.OrganizationID, in.CIDR, string(in.SNMP.Version), in.SNMP.Community, in.SNMP.Username,
		in.SNMP.AuthProto, in.SNMP.AuthPass, in.SNMP.PrivProto, in.SNMP.PrivPass,
		int(in.SNMPPort), in.TimeoutMS, in.IntervalSecs, in.Enabled,
	).Scan(&t.ID, &t.OrganizationID, &t.CIDR, &t.SNMPPort, &t.TimeoutMS, &t.IntervalSecs,
		&t.Enabled, &t.LastScannedAt, &t.LastScanError, &t.CreatedAt, &t.UpdatedAt)
	t.SNMP = in.SNMP
	return t, err
}

func (r TargetRepository) ListTargets(ctx context.Context, organizationID string) ([]Target, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("discovery target repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT id, organization_id, cidr, snmp_port, timeout_ms, interval_seconds,
			enabled, last_scanned_at, last_scan_error, created_at, updated_at
		FROM discovery_targets WHERE organization_id=$1 ORDER BY id`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.CIDR, &t.SNMPPort, &t.TimeoutMS,
			&t.IntervalSecs, &t.Enabled, &t.LastScannedAt, &t.LastScanError, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListEnabledTargets loads every enabled target across all organizations
// (with its SNMP credentials, needed to actually run the scan) -- read by
// the background AutoScanner, mirroring every other poller's
// ListAllEnabled-style query in this codebase.
func (r TargetRepository) ListEnabledTargets(ctx context.Context) ([]Target, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("discovery target repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT id, organization_id, cidr, snmp_version, snmp_community, snmp_username,
			snmp_auth_proto, snmp_auth_pass, snmp_priv_proto, snmp_priv_pass,
			snmp_port, timeout_ms, interval_seconds, last_scanned_at
		FROM discovery_targets WHERE enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Target
	for rows.Next() {
		var t Target
		var version string
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.CIDR, &version, &t.SNMP.Community, &t.SNMP.Username,
			&t.SNMP.AuthProto, &t.SNMP.AuthPass, &t.SNMP.PrivProto, &t.SNMP.PrivPass,
			&t.SNMPPort, &t.TimeoutMS, &t.IntervalSecs, &t.LastScannedAt); err != nil {
			return nil, err
		}
		t.SNMP.Version = snmp.Version(version)
		t.Enabled = true
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r TargetRepository) SetTargetEnabled(ctx context.Context, id int64, enabled bool) error {
	if r.DB == nil {
		return fmt.Errorf("discovery target repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE discovery_targets SET enabled=$2, updated_at=now() WHERE id=$1`, id, enabled)
	return err
}

func (r TargetRepository) DeleteTarget(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("discovery target repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM discovery_targets WHERE id=$1`, id)
	return err
}

// RecordScan updates a target's last-scan bookkeeping.
func (r TargetRepository) RecordScan(ctx context.Context, id int64, scanErr string) error {
	if r.DB == nil {
		return fmt.Errorf("discovery target repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE discovery_targets SET last_scanned_at=now(), last_scan_error=$2 WHERE id=$1`, id, scanErr)
	return err
}

// UpsertCandidate records (or refreshes last_seen_at on) one responsive
// host from a scheduled scan. A candidate already marked imported/ignored
// keeps that status -- reappearing on a later scan doesn't reset an
// operator's prior decision back to "new".
func (r TargetRepository) UpsertCandidate(ctx context.Context, c Candidate) error {
	if r.DB == nil {
		return fmt.Errorf("discovery target repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO discovery_candidates
			(target_id, address, system_name, sys_descr, sys_object_id, device_type, vendor)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (target_id, address) DO UPDATE SET
			system_name=EXCLUDED.system_name,
			sys_descr=EXCLUDED.sys_descr,
			sys_object_id=EXCLUDED.sys_object_id,
			device_type=EXCLUDED.device_type,
			vendor=EXCLUDED.vendor,
			last_seen_at=now()`,
		c.TargetID, c.Address, c.SystemName, c.SysDescr, c.SysObjectID, c.DeviceType, c.Vendor)
	return err
}

// ListCandidates returns every persisted candidate for a target (or every
// target belonging to an organization, if targetID is 0), optionally
// filtered to a status ("" means all).
func (r TargetRepository) ListCandidates(ctx context.Context, organizationID string, targetID int64, status string) ([]Candidate, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("discovery target repository is not initialized")
	}
	query := `
		SELECT c.id, c.target_id, c.address, c.system_name, c.sys_descr, c.sys_object_id,
			c.device_type, c.vendor, c.status, c.first_seen_at, c.last_seen_at, c.imported_device_id
		FROM discovery_candidates c
		JOIN discovery_targets t ON t.id = c.target_id
		WHERE t.organization_id=$1`
	args := []any{organizationID}
	if targetID > 0 {
		args = append(args, targetID)
		query += fmt.Sprintf(" AND c.target_id=$%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND c.status=$%d", len(args))
	}
	query += " ORDER BY c.last_seen_at DESC"

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.TargetID, &c.Address, &c.SystemName, &c.SysDescr, &c.SysObjectID,
			&c.DeviceType, &c.Vendor, &c.Status, &c.FirstSeenAt, &c.LastSeenAt, &c.ImportedDeviceID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r TargetRepository) CandidateByID(ctx context.Context, id int64) (Candidate, error) {
	if r.DB == nil {
		return Candidate{}, fmt.Errorf("discovery target repository is not initialized")
	}
	var c Candidate
	err := r.DB.QueryRow(ctx, `
		SELECT id, target_id, address, system_name, sys_descr, sys_object_id,
			device_type, vendor, status, first_seen_at, last_seen_at, imported_device_id
		FROM discovery_candidates WHERE id=$1`, id).
		Scan(&c.ID, &c.TargetID, &c.Address, &c.SystemName, &c.SysDescr, &c.SysObjectID,
			&c.DeviceType, &c.Vendor, &c.Status, &c.FirstSeenAt, &c.LastSeenAt, &c.ImportedDeviceID)
	return c, err
}

func (r TargetRepository) TargetByID(ctx context.Context, id int64) (Target, error) {
	if r.DB == nil {
		return Target{}, fmt.Errorf("discovery target repository is not initialized")
	}
	var t Target
	var version string
	err := r.DB.QueryRow(ctx, `
		SELECT id, organization_id, cidr, snmp_version, snmp_community, snmp_username,
			snmp_auth_proto, snmp_auth_pass, snmp_priv_proto, snmp_priv_pass, snmp_port
		FROM discovery_targets WHERE id=$1`, id).
		Scan(&t.ID, &t.OrganizationID, &t.CIDR, &version, &t.SNMP.Community, &t.SNMP.Username,
			&t.SNMP.AuthProto, &t.SNMP.AuthPass, &t.SNMP.PrivProto, &t.SNMP.PrivPass, &t.SNMPPort)
	t.SNMP.Version = snmp.Version(version)
	return t, err
}

// MarkCandidateStatus sets a candidate's review status ("imported" or
// "ignored"); importedDeviceID is recorded only for "imported".
func (r TargetRepository) MarkCandidateStatus(ctx context.Context, id int64, status string, importedDeviceID string) error {
	if r.DB == nil {
		return fmt.Errorf("discovery target repository is not initialized")
	}
	var idPtr *string
	if importedDeviceID != "" {
		idPtr = &importedDeviceID
	}
	_, err := r.DB.Exec(ctx, `UPDATE discovery_candidates SET status=$2, imported_device_id=$3 WHERE id=$1`, id, status, idPtr)
	return err
}
