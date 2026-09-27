-- Port/Service check monitor type (6-page rebuild, item 2.1): a new
-- monitor type alongside the existing ICMP and SNMP checks -- either a
-- plain TCP connect test, or a real HTTP(S) GET with status-code
-- validation, configurable from the Add Device popup. Defaults reproduce
-- "not configured" (off) for every existing device.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS port_check_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS port_check_protocol TEXT NOT NULL DEFAULT 'tcp' CHECK (port_check_protocol IN ('tcp','http','https'));
ALTER TABLE devices ADD COLUMN IF NOT EXISTS port_check_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS port_check_path TEXT NOT NULL DEFAULT '/';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS port_check_accepted_statuscodes TEXT NOT NULL DEFAULT '200-299';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS port_check_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (port_check_interval_seconds >= 5);
