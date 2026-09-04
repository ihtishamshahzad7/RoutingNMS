-- Add Discord as a notification channel type, ported from Uptime Kuma's
-- Discord provider (one of its most-used ones). Additive only: existing
-- channel rows and every other channel_type value are untouched, so this
-- cannot disturb any client already using webhook/slack/email/telegram/
-- pagerduty/whatsapp channels.
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_channel_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_channel_type_check
    CHECK (channel_type IN ('email','slack','webhook','pagerduty','telegram','whatsapp','discord'));
