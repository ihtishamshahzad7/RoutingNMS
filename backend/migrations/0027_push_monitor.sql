-- "Push" heartbeat monitor, ported from Uptime Kuma's "Push" monitor type:
-- the monitored thing (a cron job, a script on a device with no inbound
-- reachability) calls RoutingNMS on its own schedule via a unique per-device
-- token URL (GET /api/v1/push/{token}?status=up&msg=...) instead of
-- RoutingNMS polling it. A background sweep (piggybacked on the existing
-- ICMP poller ticker) marks the device down if no push arrives within
-- push_interval_seconds + push_grace_period_seconds.
--
-- push_token is generated on first enable (see devices.Repository.
-- UpdatePushCheck) rather than here, so it starts NULL/empty for every
-- existing row -- purely additive, nothing existing changes behavior.

ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_token TEXT UNIQUE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (push_interval_seconds >= 10);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_grace_period_seconds INTEGER NOT NULL DEFAULT 30 CHECK (push_grace_period_seconds >= 0);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_last_seen_at TIMESTAMPTZ;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_last_status TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS push_last_message TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_devices_push_enabled ON devices(push_enabled);
CREATE INDEX IF NOT EXISTS idx_devices_push_token ON devices(push_token);
