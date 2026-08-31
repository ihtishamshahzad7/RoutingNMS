-- SNMP trap reception + alert rule engine, backing internal/snmptrap.
-- ISP access gear (OLTs, routers, switches, UPS controllers, etc.) can send
-- SNMP v1/v2c/v3 traps to this NMS; received traps are stored for history
-- and matched against operator-defined rules (OID-pattern -> severity) so
-- meaningful events surface without requiring a poll cycle to catch them.

CREATE TABLE IF NOT EXISTS trap_rules (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    -- OID or OID-prefix to match against the trap's primary identifying OID
    -- (the trapOID varbind for v2c/v3, or a synthesized enterprise.specific
    -- identifier for v1). Empty string / '*' matches everything.
    oid_pattern TEXT NOT NULL DEFAULT '*',
    severity TEXT NOT NULL DEFAULT 'warning', -- info|warning|critical
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notify_email TEXT,
    notify_webhook_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS snmp_traps (
    id BIGSERIAL PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_ip TEXT NOT NULL,
    snmp_version TEXT NOT NULL, -- v1|v2c|v3
    trap_oid TEXT NOT NULL DEFAULT '',
    enterprise_oid TEXT,
    generic_trap INTEGER,
    specific_trap INTEGER,
    varbinds JSONB NOT NULL DEFAULT '[]',
    matched_rule_id BIGINT REFERENCES trap_rules(id) ON DELETE SET NULL,
    severity TEXT NOT NULL DEFAULT 'info'
);

CREATE INDEX IF NOT EXISTS idx_snmp_traps_received_at ON snmp_traps(received_at DESC);
CREATE INDEX IF NOT EXISTS idx_snmp_traps_source_ip ON snmp_traps(source_ip);
CREATE INDEX IF NOT EXISTS idx_snmp_traps_severity ON snmp_traps(severity);

-- A permissive default rule so traps are visible (severity=warning) out of
-- the box even before an operator configures anything -- mirrors the
-- "useful with zero config" approach used elsewhere in this NMS.
INSERT INTO trap_rules (name, oid_pattern, severity, enabled)
SELECT 'Catch-all (default)', '*', 'warning', TRUE
WHERE NOT EXISTS (SELECT 1 FROM trap_rules);
