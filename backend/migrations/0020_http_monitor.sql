-- HTTP(S) + keyword monitor, ported from the user's previous Uptime Kuma
-- deployment (the "http" and "keyword" monitor types). Adds an optional
-- HTTP check to any existing device -- a device can be both SNMP/ICMP
-- monitored (already supported) and HTTP monitored at the same time (e.g.
-- a router that also has a management/status web UI).

ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_check_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_url TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_expected_status INTEGER NOT NULL DEFAULT 200;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_keyword TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_timeout_ms INTEGER NOT NULL DEFAULT 5000;

CREATE INDEX IF NOT EXISTS idx_devices_http_check_enabled ON devices(http_check_enabled);
