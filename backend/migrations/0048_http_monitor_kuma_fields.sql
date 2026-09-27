-- Kuma core-parity checklist, Group A/B item 1: full HTTP(s) monitor type
-- matching Uptime Kuma's real add/edit fields, read directly from the
-- user's real Uptime Kuma source (src/pages/EditMonitor.vue) rather than
-- guessed from memory:
--   method (GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS)
--   headers (raw JSON object string, e.g. {"HeaderName": "HeaderValue"})
--   body + httpBodyEncoding (json|xml)
--   accepted_statuscodes (a list of ranges like "200-299" or exact codes)
--   maxredirects ("follow redirect" -- 0 disables following)
--   ignoreTls (skip TLS certificate verification)
--
-- Additive: the existing http_expected_status column (Feature/migration
-- 0020) is left in place and still used as a fallback exact-status-code
-- check for any device whose http_accepted_statuscodes is empty, so no
-- existing HTTP check configuration is silently broken by this migration.

ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_method TEXT NOT NULL DEFAULT 'GET';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_body TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_body_encoding TEXT NOT NULL DEFAULT 'json';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_headers TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_accepted_statuscodes TEXT NOT NULL DEFAULT '200-299';
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_max_redirects INTEGER NOT NULL DEFAULT 10;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS http_ignore_tls BOOLEAN NOT NULL DEFAULT FALSE;
