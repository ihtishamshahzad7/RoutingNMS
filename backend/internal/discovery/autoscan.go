package discovery

import (
	"context"
	"log"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/pollpool"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// AutoScanner is the "auto" half of Feature 1.2 (Device Auto-Discovery):
// a background poller, matching the shape of every other periodic poller
// in this codebase (backend/internal/ping, dnscheck, sshcheck, ...), that
// rescans every enabled discovery_targets row on its own interval and
// records newly-responsive hosts as discovery_candidates for later
// review/import -- rather than requiring an operator to press "Discover"
// every time. It is a separate, additive mechanism: the pre-existing
// manual scan-review-import flow (Manager/Job/ScanAPI/ImportAPI) is
// untouched and still works exactly as before for an ad hoc one-off scan.
type AutoScanner struct {
	Targets   TargetRepository
	Devices   devices.Repository
	Collector snmp.Collector

	// checkInterval controls how often the scheduler wakes up to see
	// whether any target is due; each target still only actually gets
	// rescanned at its own configured IntervalSecs. Defaults to 60s.
	checkInterval time.Duration
}

// Run starts the scheduling loop; call as `go scanner.Run(ctx)`. Every
// checkInterval tick, it loads the enabled targets and probes (via
// pollpool, so multiple due subnets scan concurrently rather than one
// blocking the next -- the same Feature 1.1 primitive every other poller
// in this codebase is being incrementally adopted onto) whichever ones are
// due (never scanned, or last scanned more than IntervalSecs ago).
func (s *AutoScanner) Run(ctx context.Context) {
	interval := s.checkInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *AutoScanner) tick(ctx context.Context) {
	targets, err := s.Targets.ListEnabledTargets(ctx)
	if err != nil {
		log.Printf("discovery autoscan: list targets: %v", err)
		return
	}
	var due []Target
	now := time.Now().UTC()
	for _, t := range targets {
		if t.LastScannedAt == nil || now.Sub(*t.LastScannedAt) >= time.Duration(t.IntervalSecs)*time.Second {
			due = append(due, t)
		}
	}
	pollpool.Run(ctx, due, pollpool.DefaultWorkers, s.scanTarget)
}

func (s *AutoScanner) scanTarget(ctx context.Context, t Target) {
	hosts, err := ExpandCIDR(t.CIDR)
	if err != nil {
		_ = s.Targets.RecordScan(ctx, t.ID, err.Error())
		return
	}
	scanCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	// Probe every host in this one target concurrently too -- reuses the
	// same pollpool primitive at the inner level, matching how the
	// pre-existing manual Manager.run already bounds its own fan-out with
	// a semaphore, just expressed via the shared pool now.
	pollpool.Run(scanCtx, hosts, pollpool.DefaultWorkers, func(ctx context.Context, addr string) {
		found, ok := ProbeOne(ctx, s.Collector, addr, t.SNMPPort, t.SNMP, t.TimeoutMS)
		if !ok {
			return
		}
		already, err := s.Devices.ExistsByAddress(ctx, addr)
		if err != nil {
			log.Printf("discovery autoscan: check existing device %s: %v", addr, err)
			return
		}
		if already {
			// Already a monitored device -- not a "new" candidate.
			return
		}
		if err := s.Targets.UpsertCandidate(ctx, Candidate{
			TargetID:    t.ID,
			Address:     found.Address,
			SystemName:  found.SystemName,
			SysDescr:    found.SysDescr,
			SysObjectID: found.SysObject,
			DeviceType:  found.DeviceType,
			Vendor:      found.Vendor,
		}); err != nil {
			log.Printf("discovery autoscan: upsert candidate %s: %v", addr, err)
		}
	})

	_ = s.Targets.RecordScan(ctx, t.ID, "")
}
