// Package hostmetrics implements item 3.5 of the SNMP & Syslog Monitoring
// build: periodic CPU/memory polling for every SNMP-enabled device, using
// standard HOST-RESOURCES-MIB where it's supported and vendor-specific OIDs
// (confirmed against the user's real MikroTik and Cisco IOS devices before
// this was built) where it isn't. Follows the same
// Repository/Poller/metric_samples shape as internal/ifpoll (item 3.1/3.4)
// -- a fixed global tick re-polls every enabled device each cycle, like
// internal/ping and internal/portcheck, rather than ifpoll's per-device
// interval refinement (there's no per-device host-metrics interval column,
// and this sub-item didn't ask for one).
//
// Threshold-crossing events (high_cpu/high_memory) are NOT fired from here.
// Per the 3.3a instruction not to build a parallel event system, this
// poller only writes cpu_percent/memory_percent into metric_samples;
// internal/events/scan.go's scanHostMetric watches those same samples for
// a threshold crossing and records the event, exactly like item 3.1's
// interface_transitions feed scanInterfaces.
package hostmetrics

import (
	"context"
	"log"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/pollpool"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type Poller struct {
	repo      Repository
	metrics   metricsdb.Repository
	collector snmp.Collector
}

func New(repo Repository, metrics metricsdb.Repository) *Poller {
	return &Poller{repo: repo, metrics: metrics}
}

func (p *Poller) Run(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = 60 * time.Second
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

func (p *Poller) pollOnce(ctx context.Context) {
	devices, err := p.repo.ListEnabled(ctx)
	if err != nil {
		log.Printf("hostmetrics poller: list snmp devices: %v", err)
		return
	}
	if len(devices) == 0 {
		return
	}
	pollpool.Run(ctx, devices, pollpool.DefaultWorkers, func(ctx context.Context, d EnabledDevice) {
		p.pollDevice(ctx, d)
	})
}

func (p *Poller) pollDevice(ctx context.Context, d EnabledDevice) {
	target := snmp.Target{ID: d.ID, Address: d.Address, Port: d.SNMPPort, Credentials: d.Credentials, Timeout: d.Timeout, Retries: 1}
	pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	reading, err := Poll(pollCtx, p.collector, target, d.Vendor)
	if err != nil {
		log.Printf("hostmetrics poller: poll device=%s: %v", d.ID, err)
		return
	}

	now := time.Now().UTC()
	samples := make([]metricsdb.Sample, 0, 2)
	if reading.CPUPercent != nil {
		samples = append(samples, metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "cpu_percent", Value: *reading.CPUPercent, RecordedAt: now})
	}
	if reading.MemoryPercent != nil {
		samples = append(samples, metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "memory_percent", Value: *reading.MemoryPercent, RecordedAt: now})
	}
	if len(samples) == 0 {
		return
	}
	if err := p.metrics.RecordBatch(ctx, samples); err != nil {
		log.Printf("hostmetrics poller: record metric samples device=%s: %v", d.ID, err)
	}
}
