// Package push implements the "push" heartbeat monitor type ported from the
// user's previous Uptime Kuma deployment: instead of RoutingNMS polling a
// device, the monitored thing (a cron job, a script on a device with no
// inbound reachability of its own) calls RoutingNMS on its own schedule via
// a unique per-device token URL. A background sweep marks the device down
// if no push arrives within the configured interval + grace period.
package push

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// API exposes the unauthenticated push-receive endpoint, mirroring
// provisioning.FetchAPI's "no session, token in the URL" shape.
type API struct {
	Devices devices.Repository
}

// ServeHTTP backs GET /api/v1/push/{token}?status=up&msg=... -- the URL an
// external cron job/service hits on its own schedule. Deliberately
// unauthenticated (like the public status page route): the 32-hex-char
// random token is the only credential, and there's no enumeration risk
// (lookup by exact token, no listing).
func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	dev, err := a.Devices.GetByPushToken(r.Context(), token)
	if err != nil {
		writeErr(w, http.StatusNotFound, "unknown or disabled push token")
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "up"
	}
	msg := r.URL.Query().Get("msg")
	if err := a.Devices.RecordPush(r.Context(), dev.ID, status, msg); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to record push")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Sweeper periodically checks every push-enabled device and records an
// "up"/"down" push_up metric sample based on whether a push arrived within
// interval+grace -- the down-detection mechanism for a monitor type that has
// no natural poll of its own. Piggybacks on its own ticker (started
// alongside the ICMP poller in main.go) rather than being folded into
// another poller, since its trigger condition (time since last push) is
// unrelated to any active probing.
type Sweeper struct {
	Devices devices.Repository
	Metrics metricsdb.Repository
}

// Run starts the periodic down-detection sweep.
func (s Sweeper) Run(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = 30 * time.Second
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	s.sweepOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepOnce(ctx)
		}
	}
}

func (s Sweeper) sweepOnce(ctx context.Context) {
	list, err := s.Devices.ListPushEnabled(ctx)
	if err != nil {
		log.Printf("push sweeper: list push-enabled devices: %v", err)
		return
	}
	now := time.Now().UTC()
	samples := make([]metricsdb.Sample, 0, len(list))
	for _, d := range list {
		interval := d.PushIntervalSeconds
		if interval <= 0 {
			interval = 60
		}
		grace := d.PushGracePeriodSeconds
		if grace < 0 {
			grace = 0
		}
		deadline := time.Duration(interval+grace) * time.Second

		up := 0.0
		if d.PushLastSeenAt != nil && now.Sub(*d.PushLastSeenAt) <= deadline {
			up = 1
		}
		samples = append(samples, metricsdb.Sample{
			SubjectType: "device", SubjectID: d.ID, MetricName: "push_up", Value: up, RecordedAt: now,
		})
	}
	if len(samples) > 0 {
		_ = s.Metrics.RecordBatch(ctx, samples)
	}
}
