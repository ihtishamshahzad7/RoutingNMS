ALTER TABLE olts ADD COLUMN IF NOT EXISTS organization_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_olts_org ON olts(organization_id);
