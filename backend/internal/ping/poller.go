// Package ping implements a periodic ICMP ping poller (the "pingmonitor"
// concept absorbed into RoutingNMS). It stores per-device probe history in
// the `ping_results` table (added by migration 0013) and records reachability
// + round-trip-time samples into the metric history so the frontend's
// existing MetricChart can render RTT sparklines.
//
// Unlike the TCP-only reachability probing used elsewhere in the system (which
// needs no elevated privileges), ICMP requires either CAP_NET_RAW or spawning
// a system `ping` binary. By default the poller probes via exec("ping") and
// only runs for devices with icmp_enabled=true, so the pre-existing TCP path
// remains the unprivileged default.
package ping

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// Result is the parsed outcome of a single ICMP probe.
type Result struct {
	Address     string   `json:"address"`
	Reachable   bool     `json:"reachable"`
	RTTMs       float64  `json:"rttMs"`
	JitterMs    float64  `json:"jitterMs"`
	LossPct     float64  `json:"lossPct"`
	TTL         int      `json:"ttl"`
	PacketSize  int      `json:"packetSize"`
	Count       int      `json:"count"`
	Error       string   `json:"error,omitempty"`
	ProbedAt    time.Time `json:"probedAt"`
}

// Repository persists probe results and reads the set of ICMP-enabled devices.
type Repository struct {
	DB *pgxpool.Pool
}

// ProbeResult is the stored row shape for one probe.
type ProbeResult struct {
	ID        int64     `json:"id"`
	DeviceID  int64     `json:"deviceId"`
	ProbedAt  time.Time `json:"probedAt"`
	RTTMs     *float64  `json:"rttMs,omitempty"`
	JitterMs  *float64  `json:"jitterMs,omitempty"`
	LossPct   float64   `json:"lossPct"`
	TTL       *int      `json:"ttl,omitempty"`
	Reachable bool      `json:"isReachable"`
}

// IcmpEnabledDevice is the subset of devices the poller iterates.
type IcmpEnabledDevice struct {
	ID              string
	Address         string
	IntervalSeconds int
	PacketSize      int
	Count           int
}

// ListIcmpEnabled returns every enabled device that has icmp_enabled=true.
func (r Repository) ListIcmpEnabled(ctx context.Context) ([]IcmpEnabledDevice, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("ping repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,address,icmp_interval_seconds,icmp_packet_size,icmp_count
		FROM devices WHERE enabled=true AND icmp_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IcmpEnabledDevice{}
	for rows.Next() {
		var d IcmpEnabledDevice
		if err := rows.Scan(&d.ID, &d.Address, &d.IntervalSeconds, &d.PacketSize, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Store records one probe result. rtt/jitter/ttl are pointer columns so they
// can be NULL when a probe failed.
func (r Repository) Store(ctx context.Context, res ProbeResult) error {
	if r.DB == nil {
		return fmt.Errorf("ping repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `INSERT INTO ping_results
		(device_id,probed_at,rtt_ms,jitter_ms,loss_pct,ttl,is_reachable)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		res.DeviceID, res.ProbedAt, res.RTTMs, res.JitterMs, res.LossPct, res.TTL, res.Reachable)
	return err
}

// History returns the last `limit` probe results for a device, oldest-first
// (chart/sparkline ready).
func (r Repository) History(ctx context.Context, deviceID string, limit int) ([]ProbeResult, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("ping repository is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	rows, err := r.DB.Query(ctx, `SELECT id,device_id,probed_at,rtt_ms,jitter_ms,loss_pct,ttl,is_reachable
		FROM (
			SELECT id,device_id,probed_at,rtt_ms,jitter_ms,loss_pct,ttl,is_reachable
			FROM ping_results WHERE device_id=$1
			ORDER BY probed_at DESC LIMIT $2
		) sub ORDER BY probed_at ASC`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProbeResult{}
	for rows.Next() {
		var p ProbeResult
		if err := rows.Scan(&p.ID, &p.DeviceID, &p.ProbedAt, &p.RTTMs, &p.JitterMs, &p.LossPct, &p.TTL, &p.Reachable); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PruneOlderThan deletes ping_results older than age (7 days by default).
func (r Repository) PruneOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("ping repository is not initialized")
	}
	tag, err := r.DB.Exec(ctx, `DELETE FROM ping_results WHERE probed_at < $1`, time.Now().Add(-age))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Poller owns the device probes and background goroutine.
type Poller struct {
	repo    Repository
	metrics metricsdb.Repository
	probe   ProbeFunc

	mu   sync.Mutex
	live map[string]Result // device id -> most recent probe result
}

// ProbeFunc performs one ICMP probe and returns the parsed result. Swappable
// for tests and to allow a raw-socket implementation in the future.
type ProbeFunc func(ctx context.Context, device IcmpEnabledDevice) Result

// New builds a poller; probe defaults to execPing.
func New(repo Repository, metrics metricsdb.Repository) *Poller {
	return &Poller{repo: repo, metrics: metrics, probe: ExecPing, live: map[string]Result{}}
}

// SetProbe overrides the probe function (used by tests).
func (p *Poller) SetProbe(f ProbeFunc) { p.probe = f }

// Run starts the periodic ICMP polling loop, mirroring
// devices.SamplePeriodically: a first pass immediately, then every poll tick
// (default 30s) it syncs the live device set, probes each ICMP-enabled device,
// stores results and records metric samples.
func (p *Poller) Run(ctx context.Context, pollTick time.Duration) {
	if pollTick <= 0 {
		pollTick = 30 * time.Second
	}
	ticker := time.NewTicker(pollTick)
	defer ticker.Stop()
	p.pollOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context) {
	devices, err := p.repo.ListIcmpEnabled(ctx)
	if err != nil {
		log.Printf("ping poller: list icmp devices: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, d := range devices {
		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		res := p.probe(probeCtx, d)
		cancel()

		p.mu.Lock()
		p.live[d.ID] = res
		p.mu.Unlock()

		did, err := strconv.ParseInt(d.ID, 10, 64)
		if err != nil {
			did = 0
		}
		rtt := res.RTTMs
		jitter := res.JitterMs
		ttl := res.TTL
		_ = p.repo.Store(ctx, ProbeResult{
			DeviceID: did, ProbedAt: res.ProbedAt, RTTMs: &rtt, JitterMs: &jitter,
			LossPct: res.LossPct, TTL: &ttl, Reachable: res.Reachable,
		})

		up := 0.0
		if res.Reachable {
			up = 1
		}
		_ = p.metrics.RecordBatch(ctx, []metricsdb.Sample{
			{SubjectType: "device", SubjectID: d.ID, MetricName: "icmp_loss_pct", Value: res.LossPct, RecordedAt: now},
			{SubjectType: "device", SubjectID: d.ID, MetricName: "icmp_rtt_ms", Value: res.RTTMs, RecordedAt: now},
			{SubjectType: "device", SubjectID: d.ID, MetricName: "icmp_reachable", Value: up, RecordedAt: now},
		})
	}
}

// Live returns the most recent probe result for a device id.
func (p *Poller) Live(deviceID string) (Result, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	res, ok := p.live[deviceID]
	return res, ok
}

// Force probes a single device immediately and returns the result.
func (p *Poller) Force(ctx context.Context, device IcmpEnabledDevice) Result {
	res := p.probe(ctx, device)
	p.mu.Lock()
	p.live[device.ID] = res
	p.mu.Unlock()
	return res
}

// ExecPing probes via the system `ping` binary (works without CAP_NET_RAW in
// the container if the binary is present and the device manager grants it).
// It parses both Unix `ping -c` and Windows `ping -n` output styles.
func ExecPing(ctx context.Context, d IcmpEnabledDevice) Result {
	res := Result{
		Address: d.Address, ProbedAt: time.Now().UTC(),
		PacketSize: d.PacketSize, Count: d.Count, TTL: 0,
	}
	if d.Count <= 0 {
		d.Count = 3
	}
	if d.PacketSize <= 0 {
		d.PacketSize = 56
	}
	args := []string{"-c", strconv.Itoa(d.Count), "-s", strconv.Itoa(d.PacketSize), "-W", "3", d.Address}
	// Windows uses -n instead of -c.
	if runtime.GOOS == "windows" {
		args = []string{"-n", strconv.Itoa(d.Count), "-l", strconv.Itoa(d.PacketSize), "-w", "3000", d.Address}
	}
	out, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	text := string(out)
	if err != nil {
		// Non-zero exit may still carry a loss line; try to parse it anyway.
		res.Error = err.Error()
	}
	res.LossPct = parseLoss(text)
	res.Reachable = res.LossPct < 100.0 && res.LossPct >= 0
	if avg, ok := parseAvgRTT(text); ok {
		res.RTTMs = avg
	}
	if rtts, ok := parseRTTList(text); ok {
		res.JitterMs = computeJitter(rtts)
	}
	if ttl, ok := parseTTL(text); ok {
		res.TTL = ttl
	}
	return res
}
