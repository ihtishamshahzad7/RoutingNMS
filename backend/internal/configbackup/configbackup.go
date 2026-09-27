// Package configbackup implements Feature 1.6 (Config Backup): point-in-time
// snapshots of a device's own running configuration (e.g. a RouterOS
// `/export` capture), with version history and a diff view.
//
// Scope decision, documented here rather than silently narrowed: a real
// automated *pull* of a device's config (SSH login + `/export`, or a vendor
// API) would need a new SSH client dependency (no golang.org/x/crypto/ssh in
// go.mod today, and this sandbox has no proxy.golang.org access to add and
// compile-verify one). Two ingestion paths are supported instead, both
// dependency-free:
//
//  1. Manual paste/upload — an operator pastes or uploads a config export.
//  2. Device-push — mirroring the existing `provisioning.FetchAPI` pattern
//     (shared-token device-facing endpoint, no session cookie), a RouterOS
//     device can be scheduled (via `/system scheduler` + `/tool fetch
//     upload=yes`) to POST its own `/export` output to this endpoint on a
//     schedule, with zero new backend dependencies.
//
// Restoring a stored config *back* onto a device (push/rollback) is
// explicitly out of scope for this pass (1.6b) — it is a materially riskier
// operation (an unattended config push that fails half-way can lock an
// operator out of a router) and deserves its own explicit design/approval
// rather than being bundled in here.
package configbackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	DB *pgxpool.Pool
	// MaxVersionsPerDevice caps stored history; 0 means "use the default"
	// (20). Older versions beyond the cap are pruned after each insert.
	MaxVersionsPerDevice int
}

type Backup struct {
	ID         int64     `json:"id"`
	DeviceID   string    `json:"deviceId"`
	ByteSize   int       `json:"byteSize"`
	SHA256     string    `json:"sha256"`
	Source     string    `json:"source"`
	TakenAt    time.Time `json:"takenAt"`
	ConfigText string    `json:"configText,omitempty"`
}

func (r Repository) maxVersions() int {
	if r.MaxVersionsPerDevice > 0 {
		return r.MaxVersionsPerDevice
	}
	return 20
}

// Store saves a new config snapshot for a device. If the config text is
// byte-identical to the most recent stored version for that device, no new
// row is written (dedup) and the existing latest version is returned
// instead -- this keeps a device that's scheduled to push daily from
// accumulating hundreds of identical rows when nothing actually changed.
func (r Repository) Store(ctx context.Context, deviceID, configText, source string) (Backup, error) {
	sum := sha256.Sum256([]byte(configText))
	hash := hex.EncodeToString(sum[:])

	if latest, ok, err := r.latest(ctx, deviceID); err == nil && ok && latest.SHA256 == hash {
		return latest, nil
	}

	var b Backup
	err := r.DB.QueryRow(ctx, `
		INSERT INTO device_config_backups (device_id, config_text, byte_size, sha256, source)
		VALUES ($1::bigint, $2, $3, $4, $5)
		RETURNING id, device_id::text, byte_size, sha256, source, taken_at
	`, deviceID, configText, len(configText), hash, source).
		Scan(&b.ID, &b.DeviceID, &b.ByteSize, &b.SHA256, &b.Source, &b.TakenAt)
	if err != nil {
		return Backup{}, err
	}
	b.ConfigText = configText

	if err := r.prune(ctx, deviceID); err != nil {
		return b, err
	}
	return b, nil
}

func (r Repository) prune(ctx context.Context, deviceID string) error {
	_, err := r.DB.Exec(ctx, `
		DELETE FROM device_config_backups
		WHERE device_id = $1::bigint AND id NOT IN (
			SELECT id FROM device_config_backups
			WHERE device_id = $1::bigint
			ORDER BY taken_at DESC
			LIMIT $2
		)
	`, deviceID, r.maxVersions())
	return err
}

func (r Repository) latest(ctx context.Context, deviceID string) (Backup, bool, error) {
	var b Backup
	err := r.DB.QueryRow(ctx, `
		SELECT id, device_id::text, byte_size, sha256, source, taken_at
		FROM device_config_backups WHERE device_id = $1::bigint
		ORDER BY taken_at DESC LIMIT 1
	`, deviceID).Scan(&b.ID, &b.DeviceID, &b.ByteSize, &b.SHA256, &b.Source, &b.TakenAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return Backup{}, false, nil
		}
		return Backup{}, false, err
	}
	return b, true, nil
}

// List returns version metadata (no config text, to keep the list light)
// for a device, newest first.
func (r Repository) List(ctx context.Context, deviceID string) ([]Backup, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT id, device_id::text, byte_size, sha256, source, taken_at
		FROM device_config_backups WHERE device_id = $1::bigint
		ORDER BY taken_at DESC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Backup{}
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.DeviceID, &b.ByteSize, &b.SHA256, &b.Source, &b.TakenAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Get returns one version's full config text, scoped to the given device so
// a caller can't fetch another device's backup by guessing an id.
func (r Repository) Get(ctx context.Context, deviceID string, id int64) (Backup, error) {
	var b Backup
	err := r.DB.QueryRow(ctx, `
		SELECT id, device_id::text, config_text, byte_size, sha256, source, taken_at
		FROM device_config_backups WHERE device_id = $1::bigint AND id = $2
	`, deviceID, id).Scan(&b.ID, &b.DeviceID, &b.ConfigText, &b.ByteSize, &b.SHA256, &b.Source, &b.TakenAt)
	if err != nil {
		return Backup{}, fmt.Errorf("config backup not found: %w", err)
	}
	return b, nil
}

// Delete removes a single stored version, scoped to the given device.
func (r Repository) Delete(ctx context.Context, deviceID string, id int64) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM device_config_backups WHERE device_id = $1::bigint AND id = $2`, deviceID, id)
	return err
}
