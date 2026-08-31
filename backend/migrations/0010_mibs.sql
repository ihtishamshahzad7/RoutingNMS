-- MIB manager, backing internal/mib. Operators upload vendor .mib/.my files
-- (Ubiquiti, MikroTik, HP, Cisco, etc.); each is parsed for name<->OID
-- mappings so alert rules, trap history, and the live OID tester can show
-- human-readable names instead of raw dotted OIDs.

CREATE TABLE IF NOT EXISTS mibs (
    id BIGSERIAL PRIMARY KEY,
    filename TEXT NOT NULL,
    module_name TEXT,
    raw_text TEXT NOT NULL,
    object_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS mib_objects (
    id BIGSERIAL PRIMARY KEY,
    mib_id BIGINT NOT NULL REFERENCES mibs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    oid TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_mib_objects_mib_id ON mib_objects(mib_id);
CREATE INDEX IF NOT EXISTS idx_mib_objects_oid ON mib_objects(oid);
CREATE INDEX IF NOT EXISTS idx_mib_objects_name_lower ON mib_objects (lower(name));
