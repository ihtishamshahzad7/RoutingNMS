CREATE TABLE IF NOT EXISTS olt_pons (
    id TEXT PRIMARY KEY,
    olt_id TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    onu_count INTEGER NOT NULL DEFAULT 0,
    last_seen TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_olt_pons_olt_id ON olt_pons (olt_id);
CREATE INDEX IF NOT EXISTS idx_olt_pons_status ON olt_pons (status);

CREATE TABLE IF NOT EXISTS olt_onus (
    id TEXT PRIMARY KEY,
    olt_id TEXT NOT NULL,
    pon_id TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    name TEXT,
    status TEXT NOT NULL DEFAULT 'unknown',
    los BOOLEAN NOT NULL DEFAULT FALSE,
    rx_power_dbm DOUBLE PRECISION,
    tx_power_dbm DOUBLE PRECISION,
    distance_meters INTEGER,
    last_seen TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_olt_onus_serial ON olt_onus (serial_number);
CREATE INDEX IF NOT EXISTS idx_olt_onus_olt_id ON olt_onus (olt_id);
CREATE INDEX IF NOT EXISTS idx_olt_onus_pon_id ON olt_onus (pon_id);
CREATE INDEX IF NOT EXISTS idx_olt_onus_status ON olt_onus (status);
CREATE INDEX IF NOT EXISTS idx_olt_onus_los ON olt_onus (los);
CREATE INDEX IF NOT EXISTS idx_olt_onus_last_seen ON olt_onus (last_seen);

ALTER TABLE olt_onus
    ADD CONSTRAINT fk_olt_onus_pon
    FOREIGN KEY (pon_id) REFERENCES olt_pons(id) ON DELETE CASCADE;
