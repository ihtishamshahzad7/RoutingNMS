-- Add Splunk (On-Call/VictorOps REST), Stackfield, WeCom, Feishu (Lark) and
-- Home Assistant as notification channel types, ported from Uptime Kuma's
-- respective notification providers. Additive only: existing channel rows
-- and every other channel_type value are untouched, so this cannot disturb
-- any client already using webhook/slack/email/telegram/pagerduty/whatsapp/
-- discord/teams/ntfy/gotify/pushover/matrix/google_chat/mattermost/opsgenie/
-- signal/bark/line/alerta/squadcast/pagertree channels.
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_channel_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_channel_type_check
    CHECK (channel_type IN ('email','slack','webhook','pagerduty','telegram','whatsapp','discord','teams','ntfy','gotify','pushover','matrix','google_chat','mattermost','opsgenie','signal','bark','line','alerta','squadcast','pagertree','splunk','stackfield','wecom','feishu','home_assistant'));
