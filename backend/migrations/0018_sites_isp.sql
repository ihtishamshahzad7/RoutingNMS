-- Sprint 3 — ISP features: sites, access points, customer connections.
--
-- Physical branch/site locations, wireless access points (sector/ptp/ptmp/
-- olt/cmts), and subscriber customer connections. Site scoping uses a plain
-- text tenant/organisation id (same convention as devices.organization_id).
-- Idempotent.

CREATE TABLE IF NOT EXISTS sites (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    code TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sites_tenant ON sites(tenant_id);

-- Wireless access point (linked to an optional SNMP device record and,
-- optionally, a site). ptp = point-to-point, ptmp = point-to-multipoint.
CREATE TABLE IF NOT EXISTS access_points (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    site_id BIGINT REFERENCES sites(id) ON DELETE SET NULL,
    device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL,
    ap_type TEXT NOT NULL DEFAULT 'sector'
        CHECK (ap_type IN ('sector','ptp','ptmp','olt','cmts')),
    frequency_band TEXT NOT NULL DEFAULT '',
    channel TEXT NOT NULL DEFAULT '',
    tx_power_dbm DOUBLE PRECISION,
    max_clients INTEGER,
    parent_ap_id BIGINT REFERENCES access_points(id) ON DELETE SET NULL,
    ip_address TEXT NOT NULL DEFAULT '',
    mac_address TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    footprint JSONB,
    monthly_bw_limit_gb INTEGER,
    tenant_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_access_points_site ON access_points(site_id);
CREATE INDEX IF NOT EXISTS idx_access_points_device ON access_points(device_id);

-- Subscriber connection (the CPE gateway device is referenced via device_id).
CREATE TABLE IF NOT EXISTS customer_connections (
    id BIGSERIAL PRIMARY KEY,
    customer_name TEXT NOT NULL,
    customer_code TEXT NOT NULL,
    access_point_id BIGINT REFERENCES access_points(id) ON DELETE SET NULL,
    device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL,
    plan_name TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    mac_address TEXT NOT NULL DEFAULT '',
    bandwidth_dl_mbps DOUBLE PRECISION NOT NULL DEFAULT 0,
    bandwidth_ul_mbps DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    contract_start DATE,
    contract_end DATE,
    notes TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    tenant_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (customer_code)
);
CREATE INDEX IF NOT EXISTS idx_customer_connections_ap ON customer_connections(access_point_id);
CREATE INDEX IF NOT EXISTS idx_customer_connections_device ON customer_connections(device_id);
CREATE INDEX IF NOT EXISTS idx_customer_connections_tenant ON customer_connections(tenant_id);
