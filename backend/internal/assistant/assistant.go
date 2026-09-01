package assistant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/alerts"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/alertsfeed"
)

// Answer is the deterministic, backend-grounded reply of the NOC AI assistant.
// There is deliberately no external model at runtime: every answer is built
// from live backend state (active alert feed + durable AI incidents + device
// inventory), so it stays truthful and works fully offline.
type Answer struct {
	Message   string    `json:"message"`
	Intent    string    `json:"intent"` // device_down | incidents | summary | fallback
	Sources   []string  `json:"sources"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Repository wraps the pools the assistant reads. It reuses the same repos
// that power the web UI (/api/v1/alerts/active and the incident hub) so the
// chat answers can never drift from what the NOC screens show.
type Repository struct{ DB *pgxpool.Pool }

func (r Repository) Answer(ctx context.Context, question string) (Answer, error) {
	if r.DB == nil {
		return Answer{}, fmt.Errorf("assistant repository is not initialized")
	}

	feed, err := alertsfeed.Repository{DB: r.DB}.List(ctx)
	if err != nil {
		return Answer{}, fmt.Errorf("load active alerts: %w", err)
	}
	incidents, err := alerts.Repository{DB: r.DB}.ListAiIncidents(ctx, "", "", 8)
	if err != nil {
		return Answer{}, fmt.Errorf("load incidents: %w", err)
	}
	var deviceCount int
	if err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM devices`).Scan(&deviceCount); err != nil {
		return Answer{}, fmt.Errorf("count devices: %w", err)
	}

	q := strings.ToLower(strings.TrimSpace(question))
	ans := Answer{UpdatedAt: time.Now().UTC()}

	should := func(keywords ...string) bool {
		for _, k := range keywords {
			if strings.Contains(q, k) {
				return true
			}
		}
		return false
	}

	switch {
	case should("down", "unreachable", "offline", "no response"):
		ans.Intent = "device_down"
		ans.Sources = []string{"alertsfeed.active", "devices"}
		ans.Message = r.deviceDownMessage(feed)

	case should("incident", "rca", "outage", "what happened", "root cause", "recent"):
		ans.Intent = "incidents"
		ans.Sources = []string{"ai_incidents"}
		ans.Message = r.incidentMessage(incidents)

	default:
		ans.Intent = "summary"
		ans.Sources = []string{"devices", "alertsfeed.active", "ai_incidents"}
		ans.Message = r.summaryMessage(feed, incidents, deviceCount)
	}

	return ans, nil
}

func (r Repository) deviceDownMessage(feed []alertsfeed.Active) string {
	down := make([]string, 0, 8)
	for _, a := range feed {
		if a.Severity == "critical" || a.Severity == "warning" {
			down = append(down, fmt.Sprintf("• %s — %s (%s, since %s)",
				a.Hostname, a.Message, a.Severity, a.Since.UTC().Format("15:04 UTC")))
		}
	}
	if len(down) == 0 {
		return "No devices are down or unreachable right now — every monitored device is answering probes."
	}
	head := fmt.Sprintf("%d active alert(s) on monitored devices right now:\n", len(down))
	return head + strings.Join(down[:min(len(down), 5)], "\n")
}

func (r Repository) incidentMessage(incidents []alerts.AiIncident) string {
	if len(incidents) == 0 {
		return "No incidents have been recorded yet. I'll be here as soon as the incident engine fires its first one."
	}
	open := 0
	mostRecent := ""
	for _, i := range incidents {
		if i.Status != "resolved" && i.Status != "closed" {
			open++
		}
	}
	if mostRecent == "" && len(incidents) > 0 {
		mostRecent = incidents[0].Title
	}
	head := fmt.Sprintf("%d open incident(s); %d recorded in total.\n", open, len(incidents))
	if open > 0 {
		head += fmt.Sprintf("Most recent open: %q [%s, %s severity].", mostRecent,
			incidents[0].Status, incidents[0].Severity)
	} else {
		head += fmt.Sprintf("Most recent (resolved): %q.", mostRecent)
	}
	return head
}

func (r Repository) summaryMessage(feed []alertsfeed.Active, incidents []alerts.AiIncident, deviceCount int) string {
	crit, warn := 0, 0
	for _, a := range feed {
		switch a.Severity {
		case "critical":
			crit++
		case "warning":
			warn++
		}
	}
	open := 0
	for _, i := range incidents {
		if i.Status != "resolved" && i.Status != "closed" {
			open++
		}
	}
	return fmt.Sprintf(
		"Live NOC status: %d device(s) monitored, %d active alert(s) (%d critical, %d warning), %d open incident(s).\n"+
			"You can ask me: \"which devices are down\", \"what incidents happened recently\", or \"show me the latest RCA\".",
		deviceCount, len(feed), crit, warn, open)
}

// min is Go 1.21+'s builtin; kept explicit for clarity alongside Go 1.24.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}