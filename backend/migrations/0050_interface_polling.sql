-- Item 3.1 of the SNMP & Syslog Monitoring build: periodic SNMP interface
-- (IF-MIB) polling. Extends the existing `interfaces` current-state table
-- (migration 0008, previously only written by the manual "Discover"
-- button) with the fields the periodic poller needs, adds a small
-- transitions table (one row per up/down change, never one row per poll),
-- and a per-device SNMP reachability table kept separate from per-port
-- state so "SNMP unreachable" is never confused with "port down".

ALTER TABLE interfaces ADD COLUMN IF NOT EXISTS if_speed_bps BIGINT NOT NULL DEFAULT 0;
ALTER TABLE interfaces ADD COLUMN IF NOT EXISTS counter_width SMALLINT NOT NULL DEFAULT 64 CHECK (counter_width IN (32,64));
ALTER TABLE interfaces ADD COLUMN IF NOT EXISTS in_rate_bps DOUBLE PRECISION;
ALTER TABLE interfaces ADD COLUMN IF NOT EXISTS out_rate_bps DOUBLE PRECISION;
ALTER TABLE interfaces ADD COLUMN IF NOT EXISTS counter_sampled_at TIMESTAMPTZ;
ALTER TABLE interfaces ADD COLUMN IF NOT EXISTS last_transition_at TIMESTAMPTZ;

-- Per-port up/down history. A row is inserted only when oper_up or
-- admin_up actually changes since the previous poll -- not once per poll
-- -- so this stays small even at a 60s default interval across a large
-- fleet.
CREATE TABLE IF NOT EXISTS interface_transitions (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    if_index BIGINT NOT NULL,
    oper_up BOOLEAN NOT NULL,
    admin_up BOOLEAN NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_interface_transitions_device_port ON interface_transitions(device_id, if_index, changed_at DESC);

-- Per-device SNMP reachability, tracked separately from per-port state:
-- when a device stops answering SNMP, its ports keep their last-known
-- state (reported as "unknown"/stale by the API, not forced to "down").
CREATE TABLE IF NOT EXISTS device_snmp_poll_state (
    device_id BIGINT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    reachable BOOLEAN NOT NULL DEFAULT TRUE,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT ''
);

-- Per-device SNMP interface-poll interval, same pattern as the existing
-- icmp_interval_seconds / port_check_interval_seconds columns.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS if_poll_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (if_poll_interval_seconds >= 10);
