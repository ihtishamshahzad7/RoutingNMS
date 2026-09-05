-- DNS resolution monitor, ported from Uptime Kuma's "DNS" monitor type:
-- periodically resolve a hostname against a configured record type (and,
-- optionally, a specific resolver server rather than the system default),
-- alerting if resolution fails or the answer doesn't match an expected
-- value. Purely additive/opt-in, mirroring the http_check_enabled /
-- icmp_enabled pattern -- a device can have this enabled alongside every
-- other monitor type at once.

ALTER TABLE devices ADD COLUMN IF NOT EXISTS dns_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS dns_hostname TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS dns_record_type TEXT NOT NULL DEFAULT 'A'
	CHECK (dns_record_type IN ('A','AAAA','CNAME','MX','TXT','NS','SOA'));
ALTER TABLE devices ADD COLUMN IF NOT EXISTS dns_resolver_server TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS dns_expected_answer TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS dns_interval_seconds INTEGER NOT NULL DEFAULT 60 CHECK (dns_interval_seconds >= 5);

CREATE INDEX IF NOT EXISTS idx_devices_dns_enabled ON devices(dns_enabled);
