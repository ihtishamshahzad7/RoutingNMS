-- Public status pages, ported from Uptime Kuma's flagship feature: a
-- branded, unauthenticated page (GET /api/v1/public/status/{slug}) showing
-- current up/down status for a chosen set of devices/OLTs, for sharing with
-- customers or embedding on a support site.

CREATE TABLE IF NOT EXISTS status_pages (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    published BOOLEAN NOT NULL DEFAULT TRUE,
    show_certificate_expiry BOOLEAN NOT NULL DEFAULT FALSE,
    footer_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_status_pages_tenant ON status_pages(tenant_id);

-- subject_type/subject_id mirror alertsfeed's convention (device|olt + the
-- devices/olts primary key as text), so a status page can list either kind.
CREATE TABLE IF NOT EXISTS status_page_items (
    id BIGSERIAL PRIMARY KEY,
    status_page_id BIGINT NOT NULL REFERENCES status_pages(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('device','olt')),
    subject_id TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_status_page_items_page ON status_page_items(status_page_id, position);
