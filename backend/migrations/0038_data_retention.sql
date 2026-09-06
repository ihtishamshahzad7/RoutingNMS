-- Configurable per-tenant data retention, ported from Uptime Kuma's
-- clear-old-data job (server/jobs/clear-old-data.js): a recurring background
-- job deletes heartbeat rows older than a `keepDataPeriodDays` setting
-- (default 180 days; a value < 1 disables cleanup and keeps data forever).
--
-- Kuma is single-instance and keeps that setting in a generic key/value
-- `setting` table. RoutingNMS is multi-tenant (see 0019_tenants_audit.sql),
-- so the equivalent setting is scoped per tenant rather than global. A
-- generic settings table would be overkill for the single setting this
-- feature adds -- following this codebase's existing convention of adding a
-- plain column for a per-tenant knob (see tenants.max_devices, .max_sites,
-- .plan etc. in 0019), we add one column to the existing tenants table
-- instead. Default 180 matches Kuma's default; existing tenants get the
-- default automatically (additive, non-disruptive) via DEFAULT + backfill.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS data_retention_days INTEGER NOT NULL DEFAULT 180;

-- metric_samples predates multi-tenancy (0011) and has no tenant_id column.
-- Add one (nullable-safe, default '') so newly written samples can be
-- attributed to the owning tenant (see metricsdb.Sample.TenantID) and swept
-- by internal/retention on a per-tenant basis. Samples already in the table,
-- and samples for subject types the codebase doesn't yet attribute to a
-- tenant (OLT/PON/ONU hierarchy, which itself predates multi-tenancy), keep
-- tenant_id='' and are left to the retention job's default-retention
-- fallback pass rather than a tenant-specific one -- see internal/retention.
ALTER TABLE metric_samples ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT '';

-- Composite index for the retention job's per-tenant range delete
-- (WHERE tenant_id=$1 AND recorded_at < $2), so cleanup stays an index
-- range scan instead of a full table scan as metric_samples grows. Plain
-- CREATE INDEX (not CONCURRENTLY), matching every other index-adding
-- migration in this codebase (e.g. 0037's idx_device_groups_tenant) --
-- this table is small enough today that a brief lock during migration is
-- acceptable, and staying consistent with the existing migrations avoids
-- introducing a new, one-off migration pattern.
CREATE INDEX IF NOT EXISTS idx_metric_samples_tenant_recorded
    ON metric_samples (tenant_id, recorded_at);
