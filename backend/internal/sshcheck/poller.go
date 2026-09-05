package sshcheck

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
	ID            string
	Address       string
	Port          int
	BannerKeyword string
}

// Repository reads the set of SSH-monitor-enabled devices, mirroring
// dnscheck.Repository.ListEnabled.
type Repository struct {
	DB *pgxpool.Pool
}

// ListEnabled returns every enabled device that has ssh_enabled=true.
func (r Repository) ListEnabled(ctx context.Context) ([]EnabledDevice, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("sshcheck repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,address,ssh_port,ssh_banner_keyword FROM devices WHERE enabled=true AND ssh_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnabledDevice{}
	for rows.Next() {
		var d EnabledDevice
		if err := rows.Scan(&d.ID, &d.Address, &d.Port, &d.BannerKeyword); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CheckFunc performs one SSH reachability check. Swappable for tests.
type CheckFunc func(ctx context.Context, d EnabledDevice) Result

// Poller owns the periodic SSH-check background goroutine, mirroring
// dnscheck.Poller's shape.
type Poller struct {
	repo    Repository
	metrics metricsdb.Repository
	check   CheckFunc

	mu   sync.Mutex
	live map[string]Result
}

func New(repo Repository, metrics metricsdb.Repository) *Poller {
	return &Poller{
		repo: repo, metrics: metrics, live: map[string]Result{},
		check: func(ctx context.Context, d EnabledDevice) Result {
			return Check(ctx, d.Address, d.Port, d.BannerKeyword, 5*time.Second)
		},
	}
}

func (p *Poller) SetCheck(f CheckFunc) { p.check = f }

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
		log.Printf("ssh poller: list ssh-enabled devices: %v", err)
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
		if res.Reachable {
			up = 1
		}
		_ = p.metrics.RecordBatch(ctx, []metricsdb.Sample{
			{SubjectType: "device", SubjectID: d.ID, MetricName: "ssh_up", Value: up, RecordedAt: now},
			{SubjectType: "device", SubjectID: d.ID, MetricName: "ssh_latency_ms", Value: res.LatencyMS, RecordedAt: now},
		})
	}
}

func (p *Poller) Live(deviceID string) (Result, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	res, ok := p.live[deviceID]
	return res, ok
}

func (p *Poller) Force(ctx context.Context, d EnabledDevice) Result {
	res := p.check(ctx, d)
	p.mu.Lock()
	p.live[d.ID] = res
	p.mu.Unlock()
	return res
}
