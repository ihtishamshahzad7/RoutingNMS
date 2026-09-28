package ifpoll

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/pollpool"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Poller owns the periodic SNMP interface-polling loop.
//
// Unlike ping.Poller and portcheck.Poller -- which both re-probe every
// enabled device on every global tick, and only store a per-device interval
// column without actually gating on it -- this poller enforces each
// device's own if_poll_interval_seconds: the global tick (typically much
// shorter, e.g. 15s) just sets the check granularity, and nextDue tracks
// when each device is actually next allowed to be polled. This is a
// deliberate, disclosed refinement beyond the literal ping/portcheck
// pattern, made because per-device interval control was an explicit
// requirement for this sub-item.
type Poller struct {
	repo     Repository
	collector snmp.Collector
	walk     WalkFunc

	mu      sync.Mutex
	nextDue map[string]time.Time
}

// New builds a poller; walk defaults to walkInterfaces.
func New(repo Repository) *Poller {
	return &Poller{repo: repo, walk: walkInterfaces, nextDue: map[string]time.Time{}}
}

// SetWalk overrides the SNMP walk function (used by tests).
func (p *Poller) SetWalk(f WalkFunc) { p.walk = f }

// Run starts the periodic interface-polling loop: a first pass immediately,
// then every tick it syncs the enabled-device set and polls whichever
// devices are due per their own if_poll_interval_seconds.
func (p *Poller) Run(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = 15 * time.Second
	}
	ticker := time.NewTicker(tick)
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

func (p *Poller) dueNow(deviceID string, interval time.Duration, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if due, ok := p.nextDue[deviceID]; ok && now.Before(due) {
		return false
	}
	p.nextDue[deviceID] = now.Add(interval)
	return true
}

func (p *Poller) pollOnce(ctx context.Context) {
	devices, err := p.repo.ListEnabled(ctx)
	if err != nil {
		log.Printf("ifpoll poller: list snmp devices: %v", err)
		return
	}

	now := time.Now().UTC()
	due := make([]EnabledDevice, 0, len(devices))
	for _, d := range devices {
		if p.dueNow(d.ID, time.Duration(d.IntervalSeconds)*time.Second, now) {
			due = append(due, d)
		}
	}
	if len(due) == 0 {
		return
	}

	pollpool.Run(ctx, due, pollpool.DefaultWorkers, func(ctx context.Context, d EnabledDevice) {
		p.pollDevice(ctx, d)
	})
}

func (p *Poller) pollDevice(ctx context.Context, d EnabledDevice) {
	target := snmp.Target{ID: d.ID, Address: d.Address, Port: d.SNMPPort, Credentials: d.Credentials, Timeout: d.Timeout, Retries: 1}

	pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	now := time.Now().UTC()

	ports, err := p.walk(pollCtx, p.collector, target)
	if err != nil {
		if mErr := p.repo.MarkUnreachable(ctx, d.ID, err.Error(), now); mErr != nil {
			log.Printf("ifpoll poller: mark unreachable device=%s: %v", d.ID, mErr)
		}
		return
	}

	prior, err := p.repo.PriorState(ctx, d.ID)
	if err != nil {
		log.Printf("ifpoll poller: prior state device=%s: %v", d.ID, err)
		prior = map[int64]PriorPort{}
	}

	if err := p.repo.SavePoll(ctx, d.ID, ports, prior, now); err != nil {
		log.Printf("ifpoll poller: save poll device=%s: %v", d.ID, err)
	}
}
