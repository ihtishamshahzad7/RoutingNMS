-- Persistent storage for the Workspace Topology Builder (feature 26 shipped
-- it with in-memory Zustand state only; this closes that gap). Mirrors the
-- device_groups / device_group_members shape already used elsewhere in this
-- codebase, but is its own tables since a workspace group's devices are
-- canvas nodes (with position, optionally linked to a real device/OLT), not
-- a join against the existing devices table.

CREATE TABLE IF NOT EXISTS workspace_topology_groups (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_workspace_topology_groups_tenant ON workspace_topology_groups(tenant_id);

-- id is client-generated text (the frontend already mints ids like
-- "dev-<timestamp>-<counter>" for optimistic local updates before any
-- backend existed) -- accepting it as the primary key avoids a client/server
-- id-reconciliation rewrite of the whole store.
CREATE TABLE IF NOT EXISTS workspace_topology_devices (
    id TEXT PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES workspace_topology_groups(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    address TEXT NOT NULL DEFAULT '',
    snmp_community TEXT NOT NULL DEFAULT '',
    pos_x DOUBLE PRECISION NOT NULL DEFAULT 0,
    pos_y DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- links this canvas node to a real devices.id/olts.id when the user has
    -- associated it with an actual monitored entity (CanvasDevice.linkedDeviceId
    -- in the frontend) -- not a foreign key, since it can point at either
    -- table and may reference an entity that's since been deleted.
    linked_device_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_workspace_topology_devices_group ON workspace_topology_devices(group_id);

CREATE TABLE IF NOT EXISTS workspace_topology_links (
    id TEXT PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES workspace_topology_groups(id) ON DELETE CASCADE,
    source_id TEXT NOT NULL REFERENCES workspace_topology_devices(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL REFERENCES workspace_topology_devices(id) ON DELETE CASCADE,
    source_port TEXT NOT NULL DEFAULT '',
    target_port TEXT NOT NULL DEFAULT '',
    discovered BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_workspace_topology_links_group ON workspace_topology_links(group_id);
