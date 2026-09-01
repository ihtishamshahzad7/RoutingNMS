-- Sprint 2 — AI incident persistence + root-cause analysis (RCA).
--
-- The `internal/incidents` engine is today purely in-memory and never
-- persisted. This table persists every incident (so history survives
-- restarts) and carries the AI/RCA fields a future RCA engine writes back:
-- root cause, confidence, affected services, recommended actions, impact,
-- timeline, similarity embeddings. Idempotent.

CREATE TABLE IF NOT EXISTS ai_incidents (
    id BIGSERIAL PRIMARY KEY,

    -- Identity / lifecycle (mirrors internal/incidents.Incident).
    incident_ref TEXT NOT NULL DEFAULT '',       -- caller's incident id (source:resource:code)
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved','analyzing','failed')),
    severity TEXT NOT NULL DEFAULT 'warning' CHECK (severity IN ('critical','warning','info')),
    title TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',             -- e.g. 'olt','device','syslog','trap','alert-rule'
    resource_id TEXT NOT NULL DEFAULT '',        -- device/OLT/ONU id the incident is about

    device_id BIGINT REFERENCES devices(id) ON DELETE SET NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- RCA output (written by the AI engine when analysis completes).
    root_cause TEXT,
    confidence_pct INTEGER CHECK (confidence_pct BETWEEN 0 AND 100),
    affected_services JSONB,
    recommended_actions JSONB,
    estimated_impact TEXT,
    timeline JSONB,
    rca_report JSONB,                            -- full structured RCA document
    ai_model TEXT NOT NULL DEFAULT '',
    context_tokens INTEGER NOT NULL DEFAULT 0,
    response_tokens INTEGER NOT NULL DEFAULT 0,
    similar_incident_ids JSONB,
    embedding JSONB,                             -- pgvector float[] on server; JSON blob elsewhere
    rca_completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_incidents_device ON ai_incidents(device_id);
CREATE INDEX IF NOT EXISTS idx_ai_incidents_status ON ai_incidents(status);
CREATE INDEX IF NOT EXISTS idx_ai_incidents_time ON ai_incidents(triggered_at DESC);
