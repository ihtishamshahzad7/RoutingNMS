CREATE TABLE IF NOT EXISTS olt_alerts (
    id BIGSERIAL PRIMARY KEY,
    olt_id TEXT NOT NULL,
    pon_id TEXT,
    onu_id TEXT,
    code TEXT NOT NULL,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    value DOUBLE PRECISION,
    status TEXT NOT NULL DEFAULT 'open',
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cleared_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_olt_alerts_olt_status ON olt_alerts(olt_id,status);
CREATE INDEX IF NOT EXISTS idx_olt_alerts_onu_status ON olt_alerts(onu_id,status);
CREATE INDEX IF NOT EXISTS idx_olt_alerts_last_seen ON olt_alerts(last_seen);
CREATE UNIQUE INDEX IF NOT EXISTS uq_olt_alert_open_event ON olt_alerts(olt_id,COALESCE(onu_id,''),code) WHERE status='open';
