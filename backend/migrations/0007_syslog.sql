-- Syslog message ingestion (RFC3164/5424-style UDP/TCP 514 listener), backing
-- internal/syslog. Every network device in an ISP's fleet (OLTs, routers,
-- switches, CMTS) can be pointed at this NMS as a syslog target; messages
-- land here and are queryable per-host/severity from the Syslog page.

CREATE TABLE IF NOT EXISTS syslog_messages (
    id BIGSERIAL PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_ip TEXT NOT NULL,
    facility INTEGER,
    severity INTEGER,
    hostname TEXT,
    tag TEXT,
    message TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_syslog_received_at ON syslog_messages(received_at DESC);
CREATE INDEX IF NOT EXISTS idx_syslog_source_ip ON syslog_messages(source_ip);
CREATE INDEX IF NOT EXISTS idx_syslog_severity ON syslog_messages(severity);

-- Keep the table from growing unbounded on a busy fleet -- a syslog firehose
-- with no retention policy is a classic way to fill a disk. 14 days is a
-- reasonable default for troubleshooting; operators can widen it later.
-- (Actual pruning is done by a periodic goroutine in cmd/api, not a cron
-- job, so this comment documents intent rather than being enforced here.)
