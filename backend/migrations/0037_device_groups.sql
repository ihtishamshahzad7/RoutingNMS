-- Device groups: a lightweight, named-folder concept for organizing devices,
-- ported from Uptime Kuma's per-status-page monitor grouping (a simple
-- monitor_group join table with a weight column), NOT Kuma's other grouping
-- mechanism (a "group" as a special monitor type with parent/child cascading
-- active-state) -- that model is far more invasive and out of scope here.
--
-- Distinct from tags (free-form, cross-cutting, many-to-many labels): a
-- group is an exclusive-ish organizational folder -- in this v1, a device
-- belongs to zero or one group, which is the simplest model that matches
-- Kuma's actual "organize monitors into named sections" use case.

CREATE TABLE IF NOT EXISTS device_groups (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_device_groups_tenant ON device_groups(tenant_id, sort_order);

-- subject_type/subject_id mirror tag_assignments' convention (device|olt +
-- the devices/olts primary key as text), so a group can hold either kind.
-- A subject can belong to at most one group (UNIQUE on subject alone), kept
-- simple per the design's "zero or one group" v1 scope.
CREATE TABLE IF NOT EXISTS device_group_members (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES device_groups(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('device','olt')),
    subject_id TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (subject_type, subject_id)
);

CREATE INDEX IF NOT EXISTS idx_device_group_members_group ON device_group_members(group_id, sort_order);
