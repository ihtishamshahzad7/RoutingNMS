-- Generic time-series metric storage, backing internal/metricsdb. Kept in
-- Postgres (like every other feature in this NMS) rather than requiring the
-- VictoriaMetrics container from deployments/docker-compose.yml, which
-- isn't part of the actual production deployment (deployments/ubuntu-24.04
-- runs bare systemd services, no Docker) -- so per-device/OLT charts work
-- out of the box on a plain `update.sh` deploy with no extra infrastructure.
--
-- subject_type/subject_id identify what the sample is about (e.g.
-- 'device'/<device id>, 'onu'/<onu id>, 'pon'/<pon id>) so one table can
-- back charts for every kind of monitored thing without a table per type.

CREATE TABLE IF NOT EXISTS metric_samples (
    id BIGSERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_metric_samples_lookup
    ON metric_samples (subject_type, subject_id, metric_name, recorded_at DESC);
