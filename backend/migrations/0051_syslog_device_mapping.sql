-- Item 3.2 of the SNMP & Syslog Monitoring build: map syslog messages to a
-- known device by source IP, and track whether the message's own timestamp
-- could be trusted or the receive time had to be used instead.

ALTER TABLE syslog_messages ADD COLUMN IF NOT EXISTS device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL;
ALTER TABLE syslog_messages ADD COLUMN IF NOT EXISTS message_timestamp TIMESTAMPTZ;
ALTER TABLE syslog_messages ADD COLUMN IF NOT EXISTS timestamp_estimated BOOLEAN NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS idx_syslog_device_id ON syslog_messages(device_id);
