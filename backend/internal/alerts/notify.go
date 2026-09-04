package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
