package dnscheck

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// EnabledDevice is the subset of devices the poller iterates.
type EnabledDevice struct {
	ID              string
	Hostname        string
	RecordType      string
	ResolverServer  string
	ExpectedAnswer  string
	IntervalSeconds int
}

// Repository reads the set of DNS-monitor-enabled devices, mirroring
// ping.Repository's ListIcmpEnabled.
type Repository struct {
	DB *pgxpool.Pool
}

// ListEnabled returns every enabled device that has dns_enabled=true.
func (r Repository) ListEnabled(ctx context.Context) ([]EnabledDevice, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("dnscheck repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,dns_hostname,dns_record_type,dns_resolver_server,dns_expected_answer,dns_interval_seconds
		FROM devices WHERE enabled=true AND dns_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnabledDevice{}
	for rows.Next() {
		var d EnabledDevice
		if err := rows.Scan(&d.ID, &d.Hostname, &d.RecordType, &d.ResolverServer, &d.ExpectedAnswer, &d.IntervalSeconds); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CheckFunc performs one DNS check and returns the parsed result. Swappable
// for tests.
type CheckFunc func(ctx context.Context, d EnabledDevice) Result

// Poller owns the periodic DNS-check background goroutine, mirroring
// ping.Poller's shape.
type Poller struct {
	repo    Repository
	metrics metricsdb.Repository
	check   CheckFunc

	mu   sync.Mutex
	live map[string]Result // device id -> most recent check result
}

// New builds a poller; check defaults to the package-level Check.
func New(repo Repository, metrics metricsdb.Repository) *Poller {
	return &Poller{
		repo: repo, metrics: metrics, live: map[string]Result{},
		check: func(ctx context.Context, d EnabledDevice) Result {
			return Check(ctx, d.Hostname, d.RecordType, d.ResolverServer, d.ExpectedAnswer, 5*time.Second)
		},
	}
}

// SetCheck overrides the check function (used by tests).
func (p *Poller) SetCheck(f CheckFunc) { p.check = f }

// Run starts the periodic DNS polling loop, mirroring ping.Poller.Run: a
// first pass immediately, then every poll tick (default 30s) it syncs the
// live device set and re-checks each DNS-enabled device.
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
	devices, err := p.repo.ListEnabled(ctx)
	if err != nil {
		log.Printf("dns poller: list dns-enabled devices: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, d := range devices {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		res := p.check(checkCtx, d)
		cancel()

		p.mu.Lock()
		p.live[d.ID] = res
		p.mu.Unlock()

		up := 0.0
		if res.Resolved {
			up = 1
		}
		_ = p.metrics.RecordBatch(ctx, []metricsdb.Sample{
			{SubjectType: "device", SubjectID: d.ID, MetricName: "dns_up", Value: up, RecordedAt: now},
			{SubjectType: "device", SubjectID: d.ID, MetricName: "dns_latency_ms", Value: res.LatencyMS, RecordedAt: now},
		})
	}
}

// Live returns the most recent check result for a device id.
func (p *Poller) Live(deviceID string) (Result, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	res, ok := p.live[deviceID]
	return res, ok
}

// Force checks a single device immediately and returns the result.
func (p *Poller) Force(ctx context.Context, d EnabledDevice) Result {
	res := p.check(ctx, d)
	p.mu.Lock()
	p.live[d.ID] = res
	p.mu.Unlock()
	return res
}
