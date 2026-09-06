// Package tenants provides minimal read/write access to the `tenants` table
// (added by migration 0019_tenants_audit.sql for multi-tenancy) -- today
// just the data-retention setting (migration 0038_data_retention.sql), the
// first per-tenant knob any Go code actually reads or writes. Kept small and
// focused rather than a full tenant CRUD package, since nothing else in the
// backend needs one yet.
package tenants

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultRetentionDays matches Uptime Kuma's clear-old-data job default
// (server/jobs/clear-old-data.js: keepDataPeriodDays defaults to 180).
const DefaultRetentionDays = 180

type Repository struct{ DB *pgxpool.Pool }

// RetentionDays returns how many days of metric_samples history the given
// tenant keeps. Falls back to DefaultRetentionDays if the tenant has no row
// yet (organization ids in this codebase are often used ad-hoc via
// devices.organization_id without a corresponding tenants row always having
// been created), matching Kuma's "missing setting -> default" behavior.
func (r Repository) RetentionDays(ctx context.Context, tenantID string) (int, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("tenants repository is not initialized")
	}
	var days int
	err := r.DB.QueryRow(ctx, `SELECT data_retention_days FROM tenants WHERE id=$1`, tenantID).Scan(&days)
	if err != nil {
		// No row for this tenant yet (or any other read error) -- fall back
		// to the default rather than failing the caller, mirroring Kuma's
		// "missing or unparseable setting resets to default" behavior.
		return DefaultRetentionDays, nil
	}
	return days, nil
}

// SetRetentionDays upserts the tenant's retention period. days<1 disables
// automatic cleanup for that tenant (data kept forever), matching Kuma's
// "period < 1 means disabled" semantics -- enforced again in the retention
// job itself, not just here, so any future direct DB edit is still safe.
func (r Repository) SetRetentionDays(ctx context.Context, tenantID string, days int) error {
	if r.DB == nil {
		return fmt.Errorf("tenants repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, data_retention_days)
		VALUES ($1, $1, $1, $2)
		ON CONFLICT (id) DO UPDATE SET data_retention_days = EXCLUDED.data_retention_days, updated_at = NOW()`,
		tenantID, days)
	return err
}

// AllTenantIDs returns every tenant id known to the tenants table, for the
// retention job's per-tenant sweep. Tenants that only exist implicitly
// (referenced by devices.organization_id but with no tenants row) are
// handled separately by the retention job using metric_samples.tenant_id.
func (r Repository) AllTenantIDs(ctx context.Context) ([]string, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("tenants repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id FROM tenants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
