-- Item 3.3a (SNMP & Syslog Monitoring): unified events backend. Collapses
-- device down/up (from metric_samples "up" transitions), interface down/up
-- (from item 3.1's interface_transitions), and classified syslog messages
-- into one queryable, deduplicated event log. CPU/memory events (item 3.5)
-- and any future source plug into the same table by calling
-- events.Repository.Record with a new event_type -- no schema change.

CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY,
    -- device_id is nullable: a future source might not be device-scoped,
    -- and syslog messages from an unmapped source still produce an event.
    device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL,
    -- group_id is a snapshot of the device's group membership at the time
    -- the event was first recorded (device_group_members is many:1 today --
    -- see migration 0037), so the event stays filterable by group even if
    -- membership changes later.
    group_id BIGINT REFERENCES device_groups(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('critical', 'warning', 'info')),
    message TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('snmp', 'syslog', 'poller')),
    -- Link back to the raw record this event was derived from.
    ref_table TEXT NOT NULL DEFAULT '',
    ref_id TEXT NOT NULL DEFAULT '',
    -- Deduplication: identical repeated events from the same device within
    -- EVENTS_DEDUP_WINDOW_SECONDS (default 60s) collapse into this one row.
    dedup_key TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 1,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_last_seen ON events(last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_events_device ON events(device_id);
CREATE INDEX IF NOT EXISTS idx_events_group ON events(group_id);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity);
-- Dedup lookup: "find the most recent row with this key" during Record().
CREATE INDEX IF NOT EXISTS idx_events_dedup ON events(dedup_key, last_seen DESC);
-- Free-text search (ILIKE) over the message body relies on the sequential
-- scan being bounded by the other filters (time range/device/etc.) above;
-- pg_trgm isn't enabled elsewhere in this project's migrations, so a
-- trigram index isn't added here to avoid depending on that extension.

-- Syslog classification patterns: configurable, DB-stored, editable later
-- via an admin UI -- same "rules table" precedent as internal/snmptrap's
-- trap_rules (migration 0009), but matching by regex against the message
-- body instead of by OID, since syslog text varies far more than trap OIDs.
CREATE TABLE IF NOT EXISTS syslog_event_patterns (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    -- Go regexp (RE2) matched case-insensitively against the message body.
    pattern TEXT NOT NULL,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('critical', 'warning', 'info')),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    -- Lower sort_order is checked first; first enabled match wins.
    sort_order INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_syslog_event_patterns_enabled ON syslog_event_patterns(enabled, sort_order);

-- Default patterns covering MikroTik RouterOS and Cisco IOS message styles.
-- event_type is deliberately the same generic vocabulary used for
-- poller-sourced events (interface_down/interface_up etc.) so a port flap
-- reported via syslog and one detected via SNMP polling show up as the same
-- kind of event in the unified feed, distinguished only by "source".
INSERT INTO syslog_event_patterns (name, pattern, event_type, severity, sort_order) VALUES
    ('Interface down', '(?i)(link[- ]?down|changed state to down|has (gone|gone\s+)?down)', 'interface_down', 'warning', 10),
    ('Interface up', '(?i)(link[- ]?up|changed state to up|has (gone|come)\s+up)', 'interface_up', 'info', 20),
    ('Loop detected', '(?i)loop[- ]?(detected|protect)', 'loop_detected', 'critical', 30),
    ('MAC flapping', '(?i)(mac[- ]?flap|mac address.*moved|moved from port)', 'mac_flapping', 'warning', 40),
    ('STP topology change', '(?i)(topology change|topologychange|rstp.*topology|stp.*topology)', 'stp_topology_change', 'warning', 50),
    ('Authentication failure', '(?i)(authentication fail|login fail|invalid user|bad password|access denied|AAA.*fail)', 'auth_failure', 'warning', 60),
    ('Device reboot', '(?i)(system (restart|rebooted|started|booting)|cold start|warm start|rebooting|configuration changed|SYS-5-RESTART)', 'device_reboot', 'warning', 70)
ON CONFLICT DO NOTHING;
