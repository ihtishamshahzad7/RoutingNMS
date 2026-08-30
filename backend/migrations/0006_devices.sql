CREATE TABLE IF NOT EXISTS devices (
    id BIGSERIAL PRIMARY KEY,
    organization_id TEXT NOT NULL,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    device_type TEXT NOT NULL CHECK (device_type IN ('router','switch','olt','server','other')),
    vendor TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    serial_number TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    monitoring_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (monitoring_interval_seconds >= 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_devices_org ON devices(organization_id);
CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(device_type);
CREATE INDEX IF NOT EXISTS idx_devices_enabled ON devices(enabled);
