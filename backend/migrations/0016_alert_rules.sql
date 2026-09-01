-- Sprint 2 — named, reusable alert rules (generic rules engine).
--
-- The `internal/alerts` package today holds an in-memory threshold engine
-- with no persistence, no API and no UI; only SNMP trap rules and OLT optical
-- alerts are live. This table persists named rules (threshold / absence /
-- icmp_loss / icmp_rtt) that a wired-up evaluator can fire against metric
-- samples, with for-duration, cooldown and channel fan-out. Idempotent.

CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    rule_type TEXT NOT NULL DEFAULT 'threshold'
        CHECK (rule_type IN ('threshold','absence','traps','icmp_loss','icmp_rtt')),
    -- For threshold/icmp rules: {metric, operator, threshold, unit}
    -- For trap rules:             {oid_pattern}
    condition_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    severity TEXT NOT NULL DEFAULT 'warning' CHECK (severity IN ('critical','warning','info')),
    -- Breach must persist this many seconds before firing (0 = immediate).
    for_duration_sec INTEGER NOT NULL DEFAULT 0,
    cooldown_sec INTEGER NOT NULL DEFAULT 300,
    -- JSON array of notification_channel ids to fan out to.
    notification_channel_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- Device-group tag filter or 'all'.
    device_group TEXT NOT NULL DEFAULT 'all',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(is_enabled);
