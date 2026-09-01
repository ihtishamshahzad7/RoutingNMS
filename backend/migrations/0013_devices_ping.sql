-- Sprint 0 — device monitoring extensions + ICMP ping history.
--
-- Adds per-device ICMP/syslog controls to the existing `devices` table and
-- the fine-grained `ping_results` table that backs a dedicated ICMP poller
-- (the system currently does TCP-only reachability probing). Idempotent.

-- Per-device ICMP ping controls (defaults on; mirrors "pingmonitor" concept).
ALTER TABLE devices ADD COLUMN IF NOT EXISTS icmp_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS icmp_interval_seconds INTEGER NOT NULL DEFAULT 30 CHECK (icmp_interval_seconds >= 5);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS icmp_packet_size INTEGER NOT NULL DEFAULT 56;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS icmp_count INTEGER NOT NULL DEFAULT 3;

-- Per-device syslog control (syslog receiver already exists; this lets an
-- admin opt a device's ICMP/syslog behaviour per-device).
ALTER TABLE devices ADD COLUMN IF NOT EXISTS syslog_enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- Fine-grained ICMP history, one row per probe result. Retained 7 days and
-- purged by the metric pruner (mirrors the "pingmonitor" persistence model).
CREATE TABLE IF NOT EXISTS ping_results (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    probed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rtt_ms DOUBLE PRECISION,
    jitter_ms DOUBLE PRECISION,
    loss_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    ttl INTEGER,
    is_reachable BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_ping_results_device_time
    ON ping_results (device_id, probed_at DESC);
