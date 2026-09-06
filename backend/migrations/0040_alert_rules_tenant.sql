-- Adds per-tenant scoping to alert_rules, which previously had no tenant_id
-- column at all -- an instance-wide gap surfaced by the backup/restore
-- feature: "overwrite" mode had to wipe every alert rule in the deployment
-- rather than just the importing tenant's own rules (see
-- internal/backup/backup.go's former DeleteAllRules call and its warning).
--
-- Follows the same "plain TEXT column, default ''" convention already used
-- by notification_channels.tenant_id and tags.tenant_id -- '' is the
-- "unattributed/instance-wide" bucket, matching how ListChannels(ctx, "")
-- already means "all tenants" elsewhere in this package. Existing rules
-- (created before any tenant scoping existed) keep tenant_id='' and remain
-- visible to every caller that lists with an empty tenantID, so nothing
-- already configured is hidden or orphaned by this migration.
ALTER TABLE alert_rules ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_alert_rules_tenant ON alert_rules(tenant_id);
