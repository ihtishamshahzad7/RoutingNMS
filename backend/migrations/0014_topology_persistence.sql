-- Sprint 1 — topology link + snapshot persistence.
--
-- The topology package already models nodes/links in memory and can walk the
-- LLDP-MIB; these tables let a scheduled discovery loop persist links and
-- keep time-travel snapshots of the network graph. Idempotent.

-- One link (L2/L3 adjacency) between two device records, discovered via LLDP,
-- CDP, or entered manually. Endpoints reference `devices.id`.
CREATE TABLE IF NOT EXISTS topology_links (
    id BIGSERIAL PRIMARY KEY,
    src_device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    src_interface TEXT NOT NULL DEFAULT '',
    dst_device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    dst_interface TEXT NOT NULL DEFAULT '',
    link_type TEXT NOT NULL DEFAULT 'lldp' CHECK (link_type IN ('lldp','cdp','ospf','bgp','manual')),
    bandwidth_mbps INTEGER,
    utilization_pct DOUBLE PRECISION,
    latency_ms DOUBLE PRECISION,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (src_device_id, src_interface, dst_device_id, dst_interface)
);

CREATE INDEX IF NOT EXISTS idx_topology_links_src ON topology_links(src_device_id);
CREATE INDEX IF NOT EXISTS idx_topology_links_dst ON topology_links(dst_device_id);

-- A point-in-time copy of the whole graph ({nodes:[],edges:[]} JSONB), for
-- time-travel on the topology page. Kept ~48h and pruned.
CREATE TABLE IF NOT EXISTS topology_snapshots (
    id BIGSERIAL PRIMARY KEY,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    node_count INTEGER NOT NULL DEFAULT 0,
    edge_count INTEGER NOT NULL DEFAULT 0,
    graph_json JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_topology_snapshots_time ON topology_snapshots(captured_at DESC);
