-- Two additive, opt-in alert-rule refinements ported from Uptime Kuma
-- (server/model/monitor.js): a "resend notification" interval that keeps
-- re-notifying at a fixed cadence while a breach stays open (instead of
-- firing once and going silent until recovery), and "upside down mode",
-- which inverts a rule's breach condition so it fires when the metric is
-- healthy rather than when it is breaching (e.g. alert if a honeypot
-- endpoint ever responds). Both default to the current, unchanged
-- behavior: resend_interval=0 means "notify once only" (today's only
-- behavior), upside_down=false leaves the condition as written. Idempotent.

ALTER TABLE alert_rules
    ADD COLUMN IF NOT EXISTS resend_interval INTEGER NOT NULL DEFAULT 0 CHECK (resend_interval >= 0);

ALTER TABLE alert_rules
    ADD COLUMN IF NOT EXISTS upside_down BOOLEAN NOT NULL DEFAULT false;
