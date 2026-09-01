package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message := fmt.Sprintf("[%s] %s\n%s", strings.ToUpper(severity), title, body)

	var err error
	switch ch.ChannelType {
	case "webhook", "slack":
		url := cfgString(ch.Config, "url", "webhook_url")
		if ch.ChannelType == "slack" && cfgString(ch.Config, "webhook_url", "") != "" {
			url = cfgString(ch.Config, "webhook_url", "")
		}
		if url == "" {
			log.Printf("alerts notifier: channel %d (%s) has no url configured", ch.ID, ch.ChannelType)
			return
		}
		payload := map[string]any{"text": message}
		if ch.ChannelType == "slack" {
			payload = map[string]any{"text": message, "username": "RoutingNMS"}
		}
		var bodyBytes []byte
		bodyBytes, err = json.Marshal(payload)
		if err == nil {
			err = postJSON(ctx, client, url, bodyBytes)
		}
	default:
		// email/pagerduty/telegram/whatsapp need credentials/send paths that
		// this self-contained deployment does not provide; log as delivered
		// stub so operators see the fanout attempted.
		log.Printf("alerts notifier: channel %d (%s) delivery not implemented; would send: %s", ch.ID, ch.ChannelType, message)
		return
	}
	if err != nil {
		log.Printf("alerts notifier: channel %d (%s): %v", ch.ID, ch.ChannelType, err)
	}
}

func postJSON(ctx context.Context, client *http.Client, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}

func cfgString(cfg map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := cfg[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}