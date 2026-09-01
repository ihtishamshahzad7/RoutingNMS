-- Sprint 2 — notification channels (multi-channel alert fan-out).
--
-- Per-tenant delivery endpoints for alert notifications (email, slack,
-- webhook, pagerduty, telegram, whatsapp). Secrets live in `config` JSON and
-- should be encrypted at rest; this schema only stores the container.
-- Idempotent.

-- tenant_id is a plain text scoping id (same convention as devices.organization_id),
-- not a FK, to keep this table independent of the later multi-tenancy migration.
CREATE TABLE IF NOT EXISTS notification_channels (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    channel_type TEXT NOT NULL
        CHECK (channel_type IN ('email','slack','webhook','pagerduty','telegram','whatsapp')),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant ON notification_channels(tenant_id);
