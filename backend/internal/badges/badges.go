package badges

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is a thin read-only view over data other packages already own
// (status_page_items/status_pages for the public-visibility gate,
// metric_samples for status/uptime/ping/response/cert-expiry) — badges does
// not duplicate monitoring logic or the public-status-page schema, it only
// presents already-collected data through a different (SVG, unauthenticated)
// surface.
type Repository struct{ DB *pgxpool.Pool }

// IsPublic reports whether the given device appears as an item on at least
// one *published* status page — mirroring Kuma's exact gate query
// (`monitor_group` joined to `group` filtered on `public = 1`). A device
// that doesn't exist, or exists but isn't on any published status page,
// reports false; the caller must never distinguish those two cases in the
// response it sends back (always the same grey N/A badge, HTTP 200).
func (r Repository) IsPublic(ctx context.Context, deviceID string) (bool, error) {
	if r.DB == nil {
		return false, fmt.Errorf("badges repository is not initialized")
	}
	var exists bool
	err := r.DB.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM status_page_items spi
		JOIN status_pages sp ON sp.id = spi.status_page_id
		WHERE spi.subject_type='device' AND spi.subject_id=$1 AND sp.published=true
	)`, deviceID).Scan(&exists)
	return exists, err
}

// latestMetric returns the most recent recorded value for one metric name,
// or ok=false if there is none.
func (r Repository) latestMetric(ctx context.Context, deviceID, metric string) (value float64, ok bool, err error) {
	row := r.DB.QueryRow(ctx, `SELECT value FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name=$2 ORDER BY recorded_at DESC LIMIT 1`, deviceID, metric)
	if scanErr := row.Scan(&value); scanErr != nil {
		return 0, false, nil // no rows (or a real error) -> just "no data"
	}
	return value, true, nil
}

// avgMetric returns the average value of one metric over the trailing
// window, or ok=false if there are no samples in that window.
func (r Repository) avgMetric(ctx context.Context, deviceID, metric string, since time.Duration) (value float64, ok bool, err error) {
	cutoff := time.Now().UTC().Add(-since)
	var avg *float64
	err = r.DB.QueryRow(ctx, `SELECT AVG(value) FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name=$2 AND recorded_at >= $3`, deviceID, metric, cutoff).Scan(&avg)
	if err != nil || avg == nil {
		return 0, false, err
	}
	return *avg, true, nil
}

// uptimePct computes the uptime percentage over the trailing window from
// the same "up" metric_samples series the Reachability dashboard and the
// public status-page resolver (internal/statuspage.StatusResolver) already
// read — the fraction of samples recorded with value=1, as a percentage.
// This intentionally reuses the exact metric name/table those already use
// rather than introducing a second uptime computation.
func (r Repository) uptimePct(ctx context.Context, deviceID string, since time.Duration) (pct float64, ok bool, err error) {
	cutoff := time.Now().UTC().Add(-since)
	var total, upCount int
	err = r.DB.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE value=1) FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='up' AND recorded_at >= $2`, deviceID, cutoff).Scan(&total, &upCount)
	if err != nil {
		return 0, false, err
	}
	if total == 0 {
		return 0, false, nil
	}
	return float64(upCount) / float64(total) * 100, true, nil
}

// ParseDuration accepts the small set of duration strings this feature
// documents supporting:
//
//   - ""            -> the caller's default (typically 24h)
//   - "<n>h"        -> n hours                 (e.g. "24h", "720h")
//   - "<n>d"        -> n days                  (e.g. "7d", "30d")
//   - "<n>y"        -> n years (365 days)       (e.g. "1y")
//   - "<n>"         -> a bare integer is treated as hours (e.g. "720")
//
// This covers the duration shapes Kuma's real badge routes accept
// (`:duration?` values like "24h", "30d", "1y") without pulling in a
// general-purpose duration-string parser.
func ParseDuration(s string, def time.Duration) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return def, nil
	}
	switch {
	case strings.HasSuffix(s, "y"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	case strings.HasSuffix(s, "d"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	case strings.HasSuffix(s, "h"):
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * time.Hour, nil
	default:
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * time.Hour, nil
	}
}

// Handler serves the public badge routes. None of these are registered
// behind the session/API-key auth middleware (see cmd/api/main.go) — like
// Kuma's real /api/badge/* routes, they're meant to be embedded on external
// pages, so they authenticate purely via the public-status-page visibility
// gate (IsPublic) instead of a session.
type Handler struct{ Repo Repository }

func (h Handler) writeSVG(w http.ResponseWriter, svg string) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(svg))
}

// naBadge is what every route falls back to when the gate fails (device
// missing, or not on a published status page) or the requested data simply
// doesn't exist yet (e.g. no cert-expiry sample for a non-HTTPS device) —
// always HTTP 200 with a grey "N/A" message, exactly like Kuma.
func (h Handler) naBadge(w http.ResponseWriter, label string) {
	h.writeSVG(w, Render(label, "N/A", ColorNA))
}

func labelParam(r *http.Request, def string) string {
	if v := strings.TrimSpace(r.URL.Query().Get("label")); v != "" {
		return v
	}
	return def
}

func colorParam(r *http.Request, name, def string) string {
	if v := strings.TrimSpace(r.URL.Query().Get(name)); v != "" {
		if !strings.HasPrefix(v, "#") {
			v = "#" + v
		}
		return v
	}
	return def
}

func (h Handler) gate(ctx context.Context, deviceID string) bool {
	ok, err := h.Repo.IsPublic(ctx, deviceID)
	return err == nil && ok
}

// Status serves GET /api/v1/badge/{deviceId}/status.
//
// Query params: label (default "Status"), upLabel/downLabel/pendingLabel/
// maintenanceLabel (default "Up"/"Down"/"Pending"/"Maintenance"),
// upColor/downColor/pendingColor/maintenanceColor (hex, `#` optional).
//
// RoutingNMS's device model only tracks up/down reachability (the "up"
// metric_samples series) — there is no persisted "pending" or "maintenance"
// device state distinct from up/down today, so this always resolves to
// "up", "down", or "N/A" (no recent sample). The pending/maintenance
// labels+colors are still accepted and rendered if a caller explicitly
// wants that vocabulary, but nothing in RoutingNMS currently produces those
// states.
func (h Handler) Status(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	label := labelParam(r, "Status")
	if !h.gate(r.Context(), deviceID) {
		h.naBadge(w, label)
		return
	}
	value, ok, err := h.Repo.latestMetric(r.Context(), deviceID, "up")
	if err != nil || !ok {
		h.naBadge(w, label)
		return
	}
	if value == 1 {
		h.writeSVG(w, Render(label, queryParam(r, "upLabel", "Up"), colorParam(r, "upColor", ColorUp)))
	} else {
		h.writeSVG(w, Render(label, queryParam(r, "downLabel", "Down"), colorParam(r, "downColor", ColorDown)))
	}
}

func queryParam(r *http.Request, name, def string) string {
	if v := strings.TrimSpace(r.URL.Query().Get(name)); v != "" {
		return v
	}
	return def
}

// Uptime serves GET /api/v1/badge/{deviceId}/uptime/{duration}, with
// {duration} optional (mirroring Kuma's `:duration?`) — defaults to 24h.
// See ParseDuration for accepted duration formats.
func (h Handler) Uptime(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	label := labelParam(r, "Uptime")
	dur, err := ParseDuration(r.PathValue("duration"), 24*time.Hour)
	if err != nil {
		h.naBadge(w, label)
		return
	}
	if !h.gate(r.Context(), deviceID) {
		h.naBadge(w, label)
		return
	}
	pct, ok, qerr := h.Repo.uptimePct(r.Context(), deviceID, dur)
	if qerr != nil || !ok {
		h.naBadge(w, label)
		return
	}
	color := ColorUp
	switch {
	case pct < 50:
		color = ColorDown
	case pct < 99:
		color = ColorPending
	}
	h.writeSVG(w, Render(label, fmt.Sprintf("%.2f%%", pct), colorParam(r, "color", color)))
}

// Ping serves GET /api/v1/badge/{deviceId}/ping/{duration} — the latest
// ICMP RTT sample (icmp_rtt_ms), falling back to the generic latency_ms
// sampler for devices that don't have ICMP ping enabled. The {duration}
// segment is accepted for URL-shape parity with Kuma (whose /ping badge
// also takes an optional duration, averaging over it) but since this route
// is documented as "latest", it's parsed-and-ignored beyond validating it;
// use /avg-response for an averaged value.
func (h Handler) Ping(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	label := labelParam(r, "Ping")
	if d := r.PathValue("duration"); d != "" {
		if _, err := ParseDuration(d, 24*time.Hour); err != nil {
			h.naBadge(w, label)
			return
		}
	}
	if !h.gate(r.Context(), deviceID) {
		h.naBadge(w, label)
		return
	}
	value, ok, err := h.Repo.latestMetric(r.Context(), deviceID, "icmp_rtt_ms")
	if err != nil || !ok {
		value, ok, err = h.Repo.latestMetric(r.Context(), deviceID, "latency_ms")
	}
	if err != nil || !ok {
		h.naBadge(w, label)
		return
	}
	h.writeSVG(w, Render(label, fmt.Sprintf("%.0fms", value), colorParam(r, "color", ColorInfo)))
}

// AvgResponse serves GET /api/v1/badge/{deviceId}/avg-response/{duration}
// (duration optional, default 24h) — the average response/RTT time over
// the window, preferring ICMP RTT then falling back to HTTP latency then
// generic device-check latency, whichever the device actually has samples
// for.
func (h Handler) AvgResponse(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	label := labelParam(r, "Avg Response")
	dur, err := ParseDuration(r.PathValue("duration"), 24*time.Hour)
	if err != nil {
		h.naBadge(w, label)
		return
	}
	if !h.gate(r.Context(), deviceID) {
		h.naBadge(w, label)
		return
	}
	for _, metric := range []string{"icmp_rtt_ms", "http_latency_ms", "latency_ms"} {
		value, ok, qerr := h.Repo.avgMetric(r.Context(), deviceID, metric, dur)
		if qerr == nil && ok {
			h.writeSVG(w, Render(label, fmt.Sprintf("%.0fms", value), colorParam(r, "color", ColorInfo)))
			return
		}
	}
	h.naBadge(w, label)
}

// Response serves GET /api/v1/badge/{deviceId}/response — the single most
// recent response-time sample (same underlying series as AvgResponse, just
// the latest value instead of an average).
func (h Handler) Response(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	label := labelParam(r, "Response")
	if !h.gate(r.Context(), deviceID) {
		h.naBadge(w, label)
		return
	}
	for _, metric := range []string{"icmp_rtt_ms", "http_latency_ms", "latency_ms"} {
		value, ok, qerr := h.Repo.latestMetric(r.Context(), deviceID, metric)
		if qerr == nil && ok {
			h.writeSVG(w, Render(label, fmt.Sprintf("%.0fms", value), colorParam(r, "color", ColorInfo)))
			return
		}
	}
	h.naBadge(w, label)
}

// CertExp serves GET /api/v1/badge/{deviceId}/cert-exp — days until TLS
// certificate expiry, read from the http_cert_expiry_days metric_samples
// series already recorded by devices.SamplePeriodically for HTTP(S)-checked
// devices (internal/devices/sampler.go). Devices without HTTP(S) checking
// enabled (or with no cert-expiry sample yet) show N/A.
func (h Handler) CertExp(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("deviceId")
	label := labelParam(r, "Cert Exp")
	if !h.gate(r.Context(), deviceID) {
		h.naBadge(w, label)
		return
	}
	days, ok, err := h.Repo.latestMetric(r.Context(), deviceID, "http_cert_expiry_days")
	if err != nil || !ok {
		h.naBadge(w, label)
		return
	}
	color := ColorUp
	switch {
	case days < 7:
		color = ColorDown
	case days < 30:
		color = ColorPending
	}
	h.writeSVG(w, Render(label, fmt.Sprintf("%.0f days", days), colorParam(r, "color", color)))
}
