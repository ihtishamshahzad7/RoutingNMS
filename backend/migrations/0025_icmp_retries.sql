-- ICMP "retries before down", ported from Uptime Kuma's ping monitor
-- (its "Retries" + "Heartbeat Retry Interval" fields): a single failed
-- probe cycle doesn't immediately flip a device to down/alerting -- it
-- takes N consecutive failures first, so one transient blip doesn't fire a
-- false alarm. Default 1 = fire immediately, i.e. identical to the existing
-- behaviour, so this cannot disturb any device that doesn't opt in.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS icmp_retries INTEGER NOT NULL DEFAULT 1 CHECK (icmp_retries >= 1);
