-- Free-form tags, ported from Uptime Kuma: any monitor there can carry
-- arbitrary colored tags used to filter/organize the dashboard. RoutingNMS
-- already has ISP-specific hierarchy (sites/access-points/customers), which
-- Kuma has no equivalent of, but nothing free-form -- this adds that.

CREATE TABLE IF NOT EXISTS tags (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#58A6FF',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_tags_tenant ON tags(tenant_id);

CREATE TABLE IF NOT EXISTS tag_assignments (
    id BIGSERIAL PRIMARY KEY,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('device','olt')),
    subject_id TEXT NOT NULL,
    UNIQUE (tag_id, subject_type, subject_id)
);

CREATE INDEX IF NOT EXISTS idx_tag_assignments_subject ON tag_assignments(subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_tag_assignments_tag ON tag_assignments(tag_id);
