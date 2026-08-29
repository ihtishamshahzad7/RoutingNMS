-- OLT hierarchy compatibility migration.
-- The base tables are created by 001_olt_configuration.sql; this migration
-- only adds hierarchy metadata/constraints and is safe to run afterward.

ALTER TABLE olt_pons
    ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ;

ALTER TABLE olt_onus
    ALTER COLUMN distance_meters TYPE DOUBLE PRECISION
    USING distance_meters::DOUBLE PRECISION;

ALTER TABLE olt_onus
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE olt_onus
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_olt_pons_olt_id ON olt_pons (olt_id);
CREATE INDEX IF NOT EXISTS idx_olt_pons_status ON olt_pons (status);
CREATE INDEX IF NOT EXISTS idx_olt_onus_olt_id ON olt_onus (olt_id);
CREATE INDEX IF NOT EXISTS idx_olt_onus_pon_id ON olt_onus (pon_id);
CREATE INDEX IF NOT EXISTS idx_olt_onus_status ON olt_onus (status);
CREATE INDEX IF NOT EXISTS idx_olt_onus_los ON olt_onus (los);
CREATE INDEX IF NOT EXISTS idx_olt_onus_last_seen ON olt_onus (last_seen);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_olt_pons_olt'
    ) THEN
        ALTER TABLE olt_pons
            ADD CONSTRAINT fk_olt_pons_olt
            FOREIGN KEY (olt_id) REFERENCES olts(id) ON DELETE CASCADE;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_olt_onus_pon'
    ) THEN
        ALTER TABLE olt_onus
            ADD CONSTRAINT fk_olt_onus_pon
            FOREIGN KEY (pon_id) REFERENCES olt_pons(id) ON DELETE CASCADE;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_olt_onus_serial
    ON olt_onus (olt_id, serial_number);
