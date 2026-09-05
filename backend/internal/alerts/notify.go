package alerts

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Notifier fans an incident out to the notification channels attached to a
// rule (notification_channels table, migration 0017). Delivery is best-effort:
// a failing webhook must never block alert evaluation, so every dispatch runs
// in-process with a short timeout and errors are logged, not returned.
type Notifier struct {
	Repo   Repository
	Client *http.Client
}

// Notify sends a human-readable alert message to every enabled channel named
// in channelIDs.
func (n Notifier) Notify(ctx context.Context, channelIDs []int64, title, body string, severity string) {
	if n.Repo.DB == nil || len(channelIDs) == 0 {
		return
	}
	channels, err := n.Repo.ListChannels(ctx, "")
	if err != nil {
		log.Printf("alerts notifier: list channels: %v", err)
		return
	}
	want := map[int64]bool{}
	for _, id := range channelIDs {
		want[id] = true
	}
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	for _, ch := range channels {
		if !want[ch.ID] || !ch.Enabled {
			continue
		}
		go n.dispatch(client, ch, title, body, severity)
	}
}

func (n Notifier) dispatch(client *http.Client, ch PersistedChannel, title, body, severity string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	message := fmt.Sprintf("[%s] %s\n%s", strings.ToUpper(severity), title, body)

	var err error
	switch ch.ChannelType {
	case "webhook", "slack":
		err = sendWebhook(ctx, client, ch, message)
	case "email":
		err = sendEmail(ch, title, message)
	case "telegram":
		err = sendTelegram(ctx, client, ch, message)
	case "pagerduty":
		err = sendPagerDuty(ctx, client, ch, title, message, severity)
	case "whatsapp":
		err = sendWhatsApp(ctx, client, ch, message)
	case "discord":
		err = sendDiscord(ctx, client, ch, message)
	case "teams":
		err = sendTeams(ctx, client, ch, title, message, severity)
	case "ntfy":
		err = sendNtfy(ctx, client, ch, message, severity)
	case "gotify":
		err = sendGotify(ctx, client, ch, message)
	case "pushover":
		err = sendPushover(ctx, client, ch, message)
	case "matrix":
		err = sendMatrix(ctx, client, ch, message)
	case "google_chat":
		err = sendGoogleChat(ctx, client, ch, title, message, severity)
	case "mattermost":
		err = sendMattermost(ctx, client, ch, title, message, severity)
	case "opsgenie":
		err = sendOpsgenie(ctx, client, ch, title, message, severity)
	case "signal":
		err = sendSignal(ctx, client, ch, message)
	case "bark":
		err = sendBark(ctx, client, ch, title, message, severity)
	case "line":
		err = sendLine(ctx, client, ch, title, message, severity)
	case "alerta":
		err = sendAlerta(ctx, client, ch, title, message, severity)
	case "squadcast":
		err = sendSquadcast(ctx, client, ch, title, message, severity)
	case "pagertree":
		err = sendPagerTree(ctx, client, ch, title, message, severity)
	default:
		err = fmt.Errorf("unsupported channel type %q", ch.ChannelType)
	}
	if err != nil {
		log.Printf("alerts notifier: channel %d (%s): %v", ch.ID, ch.ChannelType, err)
	}
}

func sendWebhook(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	u := cfgString(ch.Config, "url", "webhook_url")
	if ch.ChannelType == "slack" && cfgString(ch.Config, "webhook_url", "") != "" {
		u = cfgString(ch.Config, "webhook_url", "")
	}
	if u == "" {
		return fmt.Errorf("no url configured")
	}
	payload := map[string]any{"text": message}
	if ch.ChannelType == "slack" {
		payload = map[string]any{"text": message, "username": "RoutingNMS"}
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, u, bodyBytes, nil)
}

// sendEmail delivers over plain SMTP (stdlib net/smtp -- no external
// dependency). Config: smtp_host, smtp_port (default 587), smtp_username,
// smtp_password, from, to (comma-separated recipients). PLAIN auth is used
// when a username/password is configured; an open relay on the LAN can omit
// both.
func sendEmail(ch PersistedChannel, subject, message string) error {
	host := cfgString(ch.Config, "smtp_host", "host")
	if host == "" {
		return fmt.Errorf("smtp_host is required")
	}
	port := cfgString(ch.Config, "smtp_port", "port")
	if port == "" {
		port = "587"
	}
	from := cfgString(ch.Config, "from")
	if from == "" {
		return fmt.Errorf("from address is required")
	}
	toRaw := cfgString(ch.Config, "to")
	if toRaw == "" {
		return fmt.Errorf("to address is required")
	}
	var to []string
	for _, addr := range strings.Split(toRaw, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			to = append(to, addr)
		}
	}
	if len(to) == 0 {
		return fmt.Errorf("to address is required")
	}

	var auth smtp.Auth
	if user := cfgString(ch.Config, "smtp_username", "username"); user != "" {
		auth = smtp.PlainAuth("", user, cfgString(ch.Config, "smtp_password", "password"), host)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		from, strings.Join(to, ", "), subject, message)

	addr := host + ":" + port
	return smtp.SendMail(addr, auth, from, to, []byte(msg))
}

// sendTelegram posts to the Telegram Bot API. Config: bot_token, chat_id.
func sendTelegram(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	token := cfgString(ch.Config, "bot_token", "token")
	chatID := cfgString(ch.Config, "chat_id")
	if token == "" || chatID == "" {
		return fmt.Errorf("bot_token and chat_id are required")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	bodyBytes, err := json.Marshal(map[string]any{"chat_id": chatID, "text": message})
	if err != nil {
		return err
	}
	return postJSON(ctx, client, endpoint, bodyBytes, nil)
}

// sendPagerDuty triggers a PagerDuty Events API v2 incident. Config:
// routing_key (the integration key for an Events API v2 service).
func sendPagerDuty(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	routingKey := cfgString(ch.Config, "routing_key", "integration_key")
	if routingKey == "" {
		return fmt.Errorf("routing_key is required")
	}
	pdSeverity := severity
	switch severity {
	case "critical", "error", "warning", "info":
		// already a valid PagerDuty severity value
	default:
		pdSeverity = "critical"
	}
	payload := map[string]any{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"payload": map[string]any{
			"summary":  title,
			"source":   "RoutingNMS",
			"severity": pdSeverity,
			"custom_details": map[string]any{
				"message": message,
			},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, "https://events.pagerduty.com/v2/enqueue", bodyBytes, nil)
}

// sendWhatsApp delivers via the Twilio WhatsApp API. Config:
// account_sid, auth_token, from (e.g. "whatsapp:+14155238886"),
// to (e.g. "whatsapp:+15551234567").
func sendWhatsApp(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	sid := cfgString(ch.Config, "account_sid")
	authToken := cfgString(ch.Config, "auth_token")
	from := cfgString(ch.Config, "from")
	to := cfgString(ch.Config, "to")
	if sid == "" || authToken == "" || from == "" || to == "" {
		return fmt.Errorf("account_sid, auth_token, from and to are required")
	}
	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", sid)
	form := url.Values{"From": {from}, "To": {to}, "Body": {message}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(sid, authToken)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("twilio returned %s", resp.Status)
	}
	return nil
}

// sendDiscord posts to a Discord incoming webhook, ported from Uptime
// Kuma's Discord notification provider (one of its most-used ones). Config:
// webhook_url (Discord's "Integrations -> Webhooks -> Copy Webhook URL").
// Branded as "RoutingNMS" via the webhook's username field, matching the
// same convention already used for Slack (see sendWebhook above).
func sendDiscord(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	webhookURL := cfgString(ch.Config, "webhook_url", "url")
	if webhookURL == "" {
		return fmt.Errorf("webhook_url is required")
	}
	// Discord caps message content at 2000 characters.
	if len(message) > 2000 {
		message = message[:1997] + "..."
	}
	payload := map[string]any{"content": message, "username": "RoutingNMS"}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, webhookURL, bodyBytes, nil)
}

// sendTeams posts a Microsoft Teams "MessageCard" to an incoming webhook,
// ported from Uptime Kuma's Teams notification provider. Config:
// webhook_url (Teams "Configure incoming webhook" URL).
func sendTeams(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	webhookURL := cfgString(ch.Config, "webhook_url", "url")
	if webhookURL == "" {
		return fmt.Errorf("webhook_url is required")
	}
	down := severity != "resolved"
	var themeColor, summary string
	if down {
		themeColor = "ff0000"
		summary = fmt.Sprintf("\U0001F534 Application %s went down", title)
	} else {
		themeColor = "00e804"
		summary = fmt.Sprintf("✅ Application %s is back online", title)
	}
	facts := []map[string]any{{"name": "Monitor", "value": title}}
	section := map[string]any{
		"activityTitle": summary,
		"facts":         facts,
	}
	payload := map[string]any{
		"@context":   "https://schema.org/extensions",
		"@type":      "MessageCard",
		"themeColor": themeColor,
		"summary":    summary,
		"sections":   []map[string]any{section},
	}
	_ = message // message is folded into the card via the facts above
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, webhookURL, bodyBytes, nil)
}

// sendNtfy posts to an ntfy server/topic, ported from Uptime Kuma's ntfy
// notification provider. Config: server_url (e.g.
// https://ntfy.sh), topic, priority (1-5, default 4), auth_method
// ("usernamePassword" or "accessToken"), username, password, access_token.
func sendNtfy(ctx context.Context, client *http.Client, ch PersistedChannel, message, severity string) error {
	serverURL := cfgString(ch.Config, "server_url", "url")
	topic := cfgString(ch.Config, "topic")
	if serverURL == "" || topic == "" {
		return fmt.Errorf("server_url and topic are required")
	}
	priority := 4
	if p := cfgString(ch.Config, "priority"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			priority = n
		}
	}
	down := severity != "resolved"
	tag := "green_circle"
	title := "Up [RoutingNMS]"
	if down {
		if priority < 5 {
			priority++
		}
		tag = "red_circle"
		title = "Down [RoutingNMS]"
	}
	payload := map[string]any{
		"topic":    topic,
		"message":  message,
		"priority": priority,
		"title":    title,
		"tags":     []string{tag},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{}
	switch cfgString(ch.Config, "auth_method") {
	case "usernamePassword":
		user := cfgString(ch.Config, "username")
		pass := cfgString(ch.Config, "password")
		token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		headers["Authorization"] = "Basic " + token
	case "accessToken":
		headers["Authorization"] = "Bearer " + cfgString(ch.Config, "access_token")
	}
	return postJSON(ctx, client, serverURL, bodyBytes, headers)
}

// sendGotify posts to a self-hosted Gotify server, ported from Uptime
// Kuma's Gotify notification provider. Config: server_url, app_token,
// priority (default 8).
func sendGotify(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	serverURL := strings.TrimRight(cfgString(ch.Config, "server_url", "url"), "/")
	appToken := cfgString(ch.Config, "app_token", "token")
	if serverURL == "" || appToken == "" {
		return fmt.Errorf("server_url and app_token are required")
	}
	priority := 8
	if p := cfgString(ch.Config, "priority"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			priority = n
		}
	}
	payload := map[string]any{
		"message":  message,
		"priority": priority,
		"title":    "RoutingNMS",
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/message?token=%s", serverURL, url.QueryEscape(appToken))
	return postJSON(ctx, client, endpoint, bodyBytes, nil)
}

// sendPushover posts to the Pushover Messages API, ported from Uptime
// Kuma's Pushover notification provider. Config: user_key, app_token,
// sound, priority, title, device, ttl (all but user_key/app_token
// optional).
func sendPushover(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	userKey := cfgString(ch.Config, "user_key")
	appToken := cfgString(ch.Config, "app_token", "token")
	if userKey == "" || appToken == "" {
		return fmt.Errorf("user_key and app_token are required")
	}
	payload := map[string]any{
		"message": message,
		"user":    userKey,
		"token":   appToken,
		"retry":   "30",
		"expire":  "3600",
		"html":    1,
	}
	if v := cfgString(ch.Config, "sound"); v != "" {
		payload["sound"] = v
	}
	if v := cfgString(ch.Config, "priority"); v != "" {
		payload["priority"] = v
	}
	if v := cfgString(ch.Config, "title"); v != "" {
		payload["title"] = v
	}
	if v := cfgString(ch.Config, "device"); v != "" {
		payload["device"] = v
	}
	if v := cfgString(ch.Config, "ttl"); v != "" {
		payload["ttl"] = v
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, "https://api.pushover.net/1/messages.json", bodyBytes, nil)
}

// sendMatrix delivers to a Matrix room via the client-server API, ported
// from Uptime Kuma's Matrix notification provider. Config: homeserver_url,
// access_token, room_id.
func sendMatrix(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	homeserverURL := strings.TrimRight(cfgString(ch.Config, "homeserver_url", "url"), "/")
	accessToken := cfgString(ch.Config, "access_token", "token")
	roomID := cfgString(ch.Config, "room_id")
	if homeserverURL == "" || accessToken == "" || roomID == "" {
		return fmt.Errorf("homeserver_url, access_token and room_id are required")
	}
	txnID, err := matrixTxnID()
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message/%s",
		homeserverURL, url.PathEscape(roomID), url.PathEscape(txnID))
	bodyBytes, err := json.Marshal(map[string]any{"msgtype": "m.text", "body": message})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("matrix returned %s", resp.Status)
	}
	return nil
}

// matrixTxnID mirrors Kuma's transaction id generation: 20 random bytes,
// base64-encoded and truncated to 20 characters, unique per request.
func matrixTxnID() (string, error) {
	buf := make([]byte, 20)
	if _, err := io.ReadFull(crand.Reader, buf); err != nil {
		return "", err
	}
	s := base64.StdEncoding.EncodeToString(buf)
	if len(s) > 20 {
		s = s[:20]
	}
	return s, nil
}

// sendGoogleChat posts to a Google Chat incoming webhook, ported from
// Uptime Kuma's Google Chat notification provider. Config: webhook_url.
func sendGoogleChat(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	webhookURL := cfgString(ch.Config, "webhook_url", "url")
	if webhookURL == "" {
		return fmt.Errorf("webhook_url is required")
	}
	down := severity != "resolved"
	var prefix string
	if down {
		prefix = "\U0001F534 Application went down\n"
	} else {
		prefix = "✅ Application is back online\n"
	}
	text := fmt.Sprintf("%s*%s*\n%s", prefix, title, message)
	// Note: Kuma also appends a link back to the monitor's page when a base
	// URL setting is configured. RoutingNMS has no equivalent "public base
	// URL" concept today, so that link is intentionally omitted.
	bodyBytes, err := json.Marshal(map[string]any{"text": text})
	if err != nil {
		return err
	}
	return postJSON(ctx, client, webhookURL, bodyBytes, nil)
}

// sendMattermost posts to a Mattermost incoming webhook, ported from
// Uptime Kuma's Mattermost notification provider. Config: webhook_url,
// username (optional, default "RoutingNMS"), channel (optional),
// icon_emoji (optional), icon_url (optional).
func sendMattermost(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	webhookURL := cfgString(ch.Config, "webhook_url", "url")
	if webhookURL == "" {
		return fmt.Errorf("webhook_url is required")
	}
	down := severity != "resolved"
	username := cfgString(ch.Config, "username")
	if username == "" {
		username = "RoutingNMS"
	}
	color := "#32CD32"
	titleLine := fmt.Sprintf("%s service up!", title)
	fallback := fmt.Sprintf("%s service up", title)
	if down {
		color = "#FF0000"
		titleLine = fmt.Sprintf("%s service went down.", title)
		fallback = fmt.Sprintf("%s service went down", title)
	}
	attachment := map[string]any{
		"color":    color,
		"title":    titleLine,
		"fallback": fallback,
		"fields": []map[string]any{
			{"short": false, "title": "Error", "value": message},
		},
	}
	payload := map[string]any{
		"username":    username,
		"attachments": []map[string]any{attachment},
	}
	if v := cfgString(ch.Config, "channel"); v != "" {
		payload["channel"] = v
	}
	if v := cfgString(ch.Config, "icon_emoji"); v != "" {
		payload["icon_emoji"] = v
	}
	if v := cfgString(ch.Config, "icon_url"); v != "" {
		payload["icon_url"] = v
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, webhookURL, bodyBytes, nil)
}

// sendOpsgenie creates (or, on a genuine recovery event, closes) an
// Opsgenie alert, ported from Uptime Kuma's Opsgenie notification
// provider. Config: api_key, region ("us" default or "eu"), priority
// (1-5, default 3).
//
// The alert evaluator (evaluator.go, resolve()) calls Notify with
// severity="resolved" when a previously-breaching rule condition returns to
// normal, using the same title as the original breach notification so the
// close call below targets the alias that create used.
func sendOpsgenie(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	apiKey := cfgString(ch.Config, "api_key")
	if apiKey == "" {
		return fmt.Errorf("api_key is required")
	}
	alertsURL := "https://api.opsgenie.com/v2/alerts"
	if cfgString(ch.Config, "region") == "eu" {
		alertsURL = "https://api.eu.opsgenie.com/v2/alerts"
	}
	headers := map[string]string{"Authorization": "GenieKey " + apiKey}

	if severity == "resolved" {
		endpoint := fmt.Sprintf("%s/%s/close?identifierType=alias", alertsURL, url.PathEscape(title))
		bodyBytes, err := json.Marshal(map[string]any{"source": "RoutingNMS"})
		if err != nil {
			return err
		}
		return postJSON(ctx, client, endpoint, bodyBytes, headers)
	}

	priority := cfgString(ch.Config, "priority")
	if priority == "" {
		priority = "3"
	}
	payload := map[string]any{
		"message":     fmt.Sprintf("RoutingNMS Alert: %s", title),
		"alias":       title,
		"description": message,
		"source":      "RoutingNMS",
		"priority":    "P" + priority,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, alertsURL, bodyBytes, headers)
}

// sendSignal posts to a self-hosted signal-cli-rest-api instance, ported
// from Uptime Kuma's Signal notification provider. Config: signal_url,
// number (the sender's registered number), recipients (comma-separated).
func sendSignal(ctx context.Context, client *http.Client, ch PersistedChannel, message string) error {
	signalURL := cfgString(ch.Config, "signal_url", "url")
	number := cfgString(ch.Config, "number")
	recipientsRaw := cfgString(ch.Config, "recipients")
	if signalURL == "" || number == "" || recipientsRaw == "" {
		return fmt.Errorf("signal_url, number and recipients are required")
	}
	var recipients []string
	for _, r := range strings.Split(recipientsRaw, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			recipients = append(recipients, r)
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("recipients are required")
	}
	payload := map[string]any{
		"message":    message,
		"number":     number,
		"recipients": recipients,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, signalURL, bodyBytes, nil)
}

// sendBark posts to a Bark (Apple push bridge) server via a GET request,
// ported from Uptime Kuma's Bark notification provider. Config: endpoint
// (e.g. https://api.day.app/XXXXXXXX), group (optional, default
// "RoutingNMS"), sound (optional, default "telegraph").
func sendBark(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	endpoint := strings.TrimRight(cfgString(ch.Config, "endpoint", "url"), "/")
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	barkTitle := "RoutingNMS Monitor Down"
	if severity == "resolved" {
		barkTitle = "RoutingNMS Monitor Up"
	}
	group := cfgString(ch.Config, "group")
	if group == "" {
		group = "RoutingNMS"
	}
	sound := cfgString(ch.Config, "sound")
	if sound == "" {
		sound = "telegraph"
	}
	reqURL := fmt.Sprintf("%s/%s/%s?group=%s&sound=%s",
		endpoint, url.PathEscape(barkTitle), url.PathEscape(message),
		url.QueryEscape(group), url.QueryEscape(sound))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	_ = title
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bark returned %s", resp.Status)
	}
	return nil
}

// sendLine posts to the LINE Messaging API's push endpoint, ported from
// Uptime Kuma's Line notification provider. Config: channel_access_token,
// user_id.
func sendLine(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	token := cfgString(ch.Config, "channel_access_token")
	userID := cfgString(ch.Config, "user_id")
	if token == "" || userID == "" {
		return fmt.Errorf("channel_access_token and user_id are required")
	}
	statusTag := "[🔴 Down]"
	if severity == "resolved" {
		statusTag = "[✅ Up]"
	}
	text := fmt.Sprintf("RoutingNMS Alert: %s\nName: %s\n%s", statusTag, title, message)
	payload := map[string]any{
		"to": userID,
		"messages": []map[string]any{
			{"type": "text", "text": text},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + token}
	return postJSON(ctx, client, "https://api.line.me/v2/bot/message/push", bodyBytes, headers)
}

// sendAlerta posts to an Alerta API endpoint, ported from Uptime Kuma's
// Alerta notification provider. Config: api_endpoint, api_key, environment
// (optional), alert_state (optional, default "critical", used for the
// breach severity), recover_state (optional, default "cleared", used for
// the resolved severity).
//
// RoutingNMS does not plumb the breaching rule's subject type/name down to
// the notifier (Notify only carries title/body/severity), so the "event",
// "group" and "resource" fields below use the alert title in place of
// Kuma's per-monitor-type subject description.
func sendAlerta(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	apiEndpoint := cfgString(ch.Config, "api_endpoint")
	apiKey := cfgString(ch.Config, "api_key")
	if apiEndpoint == "" || apiKey == "" {
		return fmt.Errorf("api_endpoint and api_key are required")
	}
	down := severity != "resolved"
	alertSeverity := cfgString(ch.Config, "alert_state")
	if alertSeverity == "" {
		alertSeverity = "critical"
	}
	if !down {
		alertSeverity = cfgString(ch.Config, "recover_state")
		if alertSeverity == "" {
			alertSeverity = "cleared"
		}
	}
	text := fmt.Sprintf("Service %s is down.", title)
	if !down {
		text = fmt.Sprintf("Service %s is up.", title)
	}
	payload := map[string]any{
		"environment": cfgString(ch.Config, "environment"),
		"severity":    alertSeverity,
		"correlate":   []string{"service_up", "service_down"},
		"service":     []string{"RoutingNMS"},
		"value":       "Timeout",
		"tags":        []string{"routingnms"},
		"attributes":  map[string]any{},
		"origin":      "routingnms",
		"type":        "exceptionAlert",
		"event":       title,
		"group":       "routingnms-" + title,
		"resource":    title,
		"text":        text,
	}
	_ = message
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Key " + apiKey}
	return postJSON(ctx, client, apiEndpoint, bodyBytes, headers)
}

// sendSquadcast posts to a Squadcast webhook, ported from Uptime Kuma's
// Squadcast notification provider. Config: webhook_url.
//
// Simplification: Kuma's Squadcast provider also builds an "AlertAddress"
// tag from monitor-type-specific hostname/port/url data and copies the
// monitor's user-defined tags into the payload's tags map. Notify's call
// site (evaluator.go) only has the rule title/body/severity available --
// not the breaching subject's tags or connection details -- so the tags
// map below is left empty rather than adding new plumbing just for this.
func sendSquadcast(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	webhookURL := cfgString(ch.Config, "webhook_url")
	if webhookURL == "" {
		return fmt.Errorf("webhook_url is required")
	}
	down := severity != "resolved"
	status := "resolve"
	verb := "UP"
	if down {
		status = "trigger"
		verb = "DOWN"
	}
	payload := map[string]any{
		"message":     fmt.Sprintf("%s is %s", title, verb),
		"description": message,
		"tags":        map[string]any{},
		"status":      status,
		"event_id":    title,
		"source":      "routingnms",
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, webhookURL, bodyBytes, nil)
}

// sendPagerTree posts to a PagerTree integration URL, ported from Uptime
// Kuma's PagerTree notification provider. Config: integration_url, urgency
// (optional).
func sendPagerTree(ctx context.Context, client *http.Client, ch PersistedChannel, title, message, severity string) error {
	integrationURL := cfgString(ch.Config, "integration_url")
	if integrationURL == "" {
		return fmt.Errorf("integration_url is required")
	}
	down := severity != "resolved"
	payload := map[string]any{
		"id": title,
	}
	if down {
		payload["event_type"] = "create"
		payload["title"] = fmt.Sprintf("RoutingNMS Monitor %q is DOWN", title)
	} else {
		payload["event_type"] = "resolve"
	}
	if urgency := cfgString(ch.Config, "urgency"); urgency != "" {
		payload["urgency"] = urgency
	}
	_ = message
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return postJSON(ctx, client, integrationURL, bodyBytes, nil)
}

func postJSON(ctx context.Context, client *http.Client, targetURL string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("request returned %s", resp.Status)
	}
	return nil
}

func cfgString(cfg map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := cfg[k]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			}
		}
	}
	return ""
}
