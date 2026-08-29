CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('superadmin','admin','noc_manager','engineer','viewer')),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    device_type TEXT NOT NULL,
    vendor TEXT,
    model TEXT,
    serial_number TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    monitoring_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (monitoring_interval_seconds >= 5),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_devices_org ON devices(organization_id);
CREATE INDEX IF NOT EXISTS idx_devices_address ON devices(address);

CREATE TABLE IF NOT EXISTS interfaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    if_index BIGINT NOT NULL,
    name TEXT,
    description TEXT,
    admin_up BOOLEAN,
    oper_up BOOLEAN,
    speed_bps BIGINT,
    UNIQUE(device_id, if_index)
);

CREATE INDEX IF NOT EXISTS idx_interfaces_device ON interfaces(device_id);

CREATE TABLE IF NOT EXISTS olts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL UNIQUE REFERENCES devices(id) ON DELETE CASCADE,
    vendor TEXT NOT NULL,
    model TEXT,
    serial_number TEXT
);

CREATE TABLE IF NOT EXISTS pon_ports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    olt_id UUID NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    if_index BIGINT,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    UNIQUE(olt_id, name)
);

CREATE TABLE IF NOT EXISTS onus (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pon_port_id UUID NOT NULL REFERENCES pon_ports(id) ON DELETE CASCADE,
    serial_number TEXT NOT NULL,
    name TEXT,
    status TEXT NOT NULL DEFAULT 'unknown',
    los BOOLEAN NOT NULL DEFAULT false,
    rx_power_dbm DOUBLE PRECISION,
    tx_power_dbm DOUBLE PRECISION,
    last_seen TIMESTAMPTZ,
    UNIQUE(pon_port_id, serial_number)
);

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
    severity TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved')),
    rule_key TEXT NOT NULL,
    title TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_alerts_org_status ON alerts(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_alerts_device_status ON alerts(device_id, status);

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    root_cause TEXT,
    impact_count BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_incidents_org_status ON incidents(organization_id, status);
