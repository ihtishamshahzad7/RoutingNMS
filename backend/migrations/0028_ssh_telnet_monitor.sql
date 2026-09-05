-- SSH and Telnet reachability monitors, following the exact same
-- opt-in-per-device pattern as the HTTP/DNS/push monitor types
-- (migrations 0026/0027): a TCP-connect check to the configured port,
-- with an optional one-shot banner read compared against a keyword
-- (SSH servers send an identification banner immediately on connect;
-- Telnet servers often send IAC negotiation bytes or a login prompt).
-- Purely additive, safe defaults, no existing behavior changes.

ALTER TABLE devices ADD COLUMN IF NOT EXISTS ssh_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS ssh_port INTEGER NOT NULL DEFAULT 22 CHECK (ssh_port > 0 AND ssh_port <= 65535);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS ssh_banner_keyword TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS ssh_timeout_ms INTEGER NOT NULL DEFAULT 5000 CHECK (ssh_timeout_ms > 0);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS ssh_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (ssh_interval_seconds >= 5);

ALTER TABLE devices ADD COLUMN IF NOT EXISTS telnet_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS telnet_port INTEGER NOT NULL DEFAULT 23 CHECK (telnet_port > 0 AND telnet_port <= 65535);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS telnet_banner_keyword TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS telnet_timeout_ms INTEGER NOT NULL DEFAULT 5000 CHECK (telnet_timeout_ms > 0);
ALTER TABLE devices ADD COLUMN IF NOT EXISTS telnet_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (telnet_interval_seconds >= 5);

CREATE INDEX IF NOT EXISTS idx_devices_ssh_enabled ON devices(ssh_enabled);
CREATE INDEX IF NOT EXISTS idx_devices_telnet_enabled ON devices(telnet_enabled);
