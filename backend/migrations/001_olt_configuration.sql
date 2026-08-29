CREATE TABLE IF NOT EXISTS olts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    vendor TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    serial TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    snmp_version TEXT NOT NULL DEFAULT '2c',
    snmp_community TEXT NOT NULL DEFAULT '',
    snmp_username TEXT NOT NULL DEFAULT '',
    snmp_auth_protocol TEXT NOT NULL DEFAULT '',
    snmp_auth_password TEXT NOT NULL DEFAULT '',
    snmp_priv_protocol TEXT NOT NULL DEFAULT '',
    snmp_priv_password TEXT NOT NULL DEFAULT '',
    poll_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (poll_interval_seconds >= 30),
    profile_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_olts_enabled ON olts(enabled);
CREATE INDEX IF NOT EXISTS idx_olts_vendor_model ON olts(vendor, model);

CREATE TABLE IF NOT EXISTS olt_pons (
    id TEXT PRIMARY KEY,
    olt_id TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    onu_count INTEGER NOT NULL DEFAULT 0,
    UNIQUE (olt_id, name)
);
CREATE INDEX IF NOT EXISTS idx_olt_pons_olt ON olt_pons(olt_id);

CREATE TABLE IF NOT EXISTS olt_onus (
    id TEXT PRIMARY KEY,
    olt_id TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    pon_id TEXT NOT NULL REFERENCES olt_pons(id) ON DELETE CASCADE,
    serial_number TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    los BOOLEAN NOT NULL DEFAULT FALSE,
    rx_power_dbm DOUBLE PRECISION,
    tx_power_dbm DOUBLE PRECISION,
    distance_meters DOUBLE PRECISION,
    last_seen TIMESTAMPTZ,
    UNIQUE (olt_id, serial_number)
);
CREATE INDEX IF NOT EXISTS idx_olt_onus_pon ON olt_onus(pon_id);
CREATE INDEX IF NOT EXISTS idx_olt_onus_last_seen ON olt_onus(last_seen);
