-- Item 3.5 (SNMP & Syslog Monitoring): configurable per-device CPU/memory
-- alert thresholds. No new metric storage is added -- internal/hostmetrics'
-- poller writes cpu_percent/memory_percent into the existing metric_samples
-- table exactly like item 3.4's per-port rates, and
-- internal/events/scan.go's scanHostMetric compares each new sample
-- against these columns to fire high_cpu/high_memory events into the same
-- unified events table from 3.3a -- not a separate alert path.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS cpu_alert_threshold_pct INTEGER NOT NULL DEFAULT 85;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS memory_alert_threshold_pct INTEGER NOT NULL DEFAULT 90;
