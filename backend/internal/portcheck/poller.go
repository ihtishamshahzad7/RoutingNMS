package portcheck

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// EnabledDevice is the subset of devices the poller iterates.
type EnabledDevice struct {
	ID             string
	OrganizationID string
	Address        string
	Protocol       string
	Port           int
	Path           string
	AcceptedCodes  []string
}

// Repository reads the set of Port/Service-check-enabled devices,
// mirroring sshcheck.Repository.ListEnabled.
type Repository struct {
	DB *pgxpool.Pool
}

// ListEnabled returns every enabled device that has port_check_enabled=true.
func (r Repository) ListEnabled(ctx context.Context) ([]EnabledDevice, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("portcheck repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,address,port_check_protocol,port_check_port,port_check_path,port_check_accepted_statuscodes
		FROM devices WHERE enabled=true AND port_check_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnabledDevice{}
	for rows.Next() {
		var d EnabledDevice
		var codes string
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Address, &d.Protocol, &d.Port, &d.Path, &codes); err != nil {
			return nil, err
		}
		for _, c := range strings.Split(codes, ",") {
			if c = strings.TrimSpace(c); c != "" {
				d.AcceptedCodes = append(d.AcceptedCodes, c)
			}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CheckFunc performs one Port/Service check. Swappable for tests.
type CheckFunc func(ctx context.Context, d EnabledDevice) Result

// Poller owns the periodic Port/Service-check background goroutine,
// mirroring sshcheck.Poller's shape.
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
			return Check(ctx, d.Protocol, d.Address, d.Port, d.Path, d.AcceptedCodes, 5*time.Second)
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
		log.Printf("port-check poller: list port-check-enabled devices: %v", err)
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
			{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "port_check_up", Value: up, RecordedAt: now},
			{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "port_check_latency_ms", Value: res.LatencyMS, RecordedAt: now},
		})
	}
}

func (p *Poller) Live(deviceID string) (Result, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	res, ok := p.live[deviceID]
	return res, ok
}

// LiveAll returns a snapshot of every device's most recent result -- used by
// the Connectivity Monitoring list (item 2.2) to show Port/Service status
// side by side with ICMP status without one HTTP round-trip per device.
func (p *Poller) LiveAll() map[string]Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]Result, len(p.live))
	for k, v := range p.live {
		out[k] = v
	}
	return out
}

func (p *Poller) Force(ctx context.Context, d EnabledDevice) Result {
	res := p.check(ctx, d)
	p.mu.Lock()
	p.live[d.ID] = res
	p.mu.Unlock()
	return res
}
