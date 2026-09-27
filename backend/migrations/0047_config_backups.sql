-- Feature 1.6 (Config Backup): stores point-in-time snapshots of a device's
-- own running configuration (e.g. a RouterOS `/export` capture), distinct
-- from feature 16's backup/restore which exports RoutingNMS's own app
-- state. Idempotent, matching every prior migration in this repo.
--
-- device_id is BIGINT (not TEXT) to match devices.id's real column type --
-- confirmed by the first real deploy of this migration failing with
-- "Key columns device_id and id are of incompatible types: text and
-- bigint" (devices.id is bigint; unlike olts.id, which really is text --
-- see the working-pattern note in claude/roadmap.md about this exact
-- pitfall, previously hit by features 29/30). The Go repository layer
-- still treats device ids as strings everywhere else in this codebase, so
-- queries cast explicitly ($1::bigint on the way in, device_id::text on
-- the way out) rather than changing that convention.

CREATE TABLE IF NOT EXISTS device_config_backups (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    config_text TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual', -- 'manual' | 'device-push'
    taken_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_device_config_backups_device_taken
    ON device_config_backups (device_id, taken_at DESC);

-- Dedup guard support: quickly check the most recent backup's hash before
-- storing an identical unchanged config again.
CREATE INDEX IF NOT EXISTS idx_device_config_backups_device_sha
    ON device_config_backups (device_id, sha256);
