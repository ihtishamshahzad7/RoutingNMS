-- Add Rocket.Chat, DingDing (DingTalk), Kook, LunaSea, ServerChan and GoAlert
-- as notification channel types, ported from Uptime Kuma's respective
-- notification providers. Additive only: existing channel rows and every
-- other channel_type value are untouched, so this cannot disturb any client
-- already using webhook/slack/email/telegram/pagerduty/whatsapp/discord/
-- teams/ntfy/gotify/pushover/matrix/google_chat/mattermost/opsgenie/signal/
-- bark/line/alerta/squadcast/pagertree/splunk/stackfield/wecom/feishu/
-- home_assistant channels.
--
-- Note: Nostr is deliberately NOT included here -- it requires secp256k1 key
-- signing, NIP-04 encryption, bech32 npub/nsec decoding and raw WebSocket
-- relay connections, which is out of scope for this additive batch.
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_channel_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_channel_type_check
    CHECK (channel_type IN ('email','slack','webhook','pagerduty','telegram','whatsapp','discord','teams','ntfy','gotify','pushover','matrix','google_chat','mattermost','opsgenie','signal','bark','line','alerta','squadcast','pagertree','splunk','stackfield','wecom','feishu','home_assistant','rocket_chat','dingding','kook','lunasea','serverchan','goalert'));
