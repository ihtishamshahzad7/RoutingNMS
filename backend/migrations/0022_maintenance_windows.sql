-- Maintenance windows, ported from Uptime Kuma: suppress alerts for chosen
-- devices/OLTs during planned downtime, so a scheduled truck roll or a
-- firmware upgrade doesn't page anyone.
--
-- Two strategies, matching Kuma's most-used ones:
--   'single'    -- one-off window: starts_at .. ends_at (both set)
--   'recurring' -- a weekly recurring window: days_of_week + start_time_of_day
--                  + duration_minutes, in `timezone` (starts_at/ends_at unused)

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    strategy TEXT NOT NULL CHECK (strategy IN ('single','recurring')),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    days_of_week INTEGER[] NOT NULL DEFAULT '{}', -- 0=Sunday .. 6=Saturday, recurring only
    start_time_of_day TIME, -- recurring only, interpreted in `timezone`
    duration_minutes INTEGER NOT NULL DEFAULT 60,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_maintenance_windows_tenant ON maintenance_windows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_windows_active ON maintenance_windows(active);

CREATE TABLE IF NOT EXISTS maintenance_window_items (
    id BIGSERIAL PRIMARY KEY,
    maintenance_window_id BIGINT NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('device','olt')),
    subject_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_maintenance_window_items_window ON maintenance_window_items(maintenance_window_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_window_items_subject ON maintenance_window_items(subject_type, subject_id);
