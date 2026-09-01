-- Sprint 4 — multi-tenancy.
--
-- Tenants scope devices/sites/channels/customers. Kept aligned with the
-- existing convention of scoping by a plain-text id (devices.organization_id
-- and the tenant_id columns added in earlier migrations), so the Tenant row
-- and the scoped rows share the same id domain without a hard FK dependency.
-- Idempotent.

CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,               -- matches devices.organization_id / *.tenant_id
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    plan TEXT NOT NULL DEFAULT 'free',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    max_devices INTEGER NOT NULL DEFAULT 0,
    max_sites INTEGER NOT NULL DEFAULT 0,
    api_key TEXT NOT NULL DEFAULT '',
    smtp_host TEXT NOT NULL DEFAULT '',
    smtp_port INTEGER NOT NULL DEFAULT 587,
    smtp_user TEXT NOT NULL DEFAULT '',
    -- smtp secret should be encrypted at rest; column exists as container.
    smtp_pass TEXT NOT NULL DEFAULT '',
    webhook_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (slug),
    UNIQUE (api_key) WHERE api_key <> ''
);

-- Audit log: every write operation (create/update/delete/login/logout/
-- provision/ack_alert) across the system.
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL DEFAULT '',
    old_values JSONB,
    new_values JSONB,
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_time ON audit_logs(created_at DESC);
