package topolinks

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Poller periodically resolves each topology link's two named interfaces
// via SNMP (ifDescr/ifName -> ifOperStatus, IF-MIB semantics: 1=up, 2=down,
// others treated as down) and records the result as a metric sample plus an
// in-memory live status, mirroring dnscheck.Poller/sshcheck.Poller's shape.
type Poller struct {
	Links     Repository
	Devices   devices.Repository
	Metrics   metricsdb.Repository
	Collector snmp.Collector

	mu   sync.Mutex
	live map[string]LinkStatus
}

func New(links Repository, devicesRepo devices.Repository, metrics metricsdb.Repository) *Poller {
	return &Poller{
		Links: links, Devices: devicesRepo, Metrics: metrics,
		Collector: snmp.Collector{}, live: map[string]LinkStatus{},
	}
}

func (p *Poller) Run(ctx context.Context, pollTick time.Duration) {
	if pollTick <= 0 {
		pollTick = 60 * time.Second
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

// discoveryCache avoids re-walking a device's ifTable once per link when
// several links in the same poll cycle share an endpoint device.
type discoveryCache struct {
	collector snmp.Collector
	devices   devices.Repository
	results   map[string]snmp.DiscoveryResult
	errs      map[string]error
}

func (c *discoveryCache) get(ctx context.Context, deviceID string) (snmp.DiscoveryResult, error) {
	if res, ok := c.results[deviceID]; ok {
		return res, c.errs[deviceID]
	}
	target, err := c.devices.DiscoveryTarget(ctx, deviceID)
	if err != nil {
		c.errs[deviceID] = err
		return snmp.DiscoveryResult{}, err
	}
	snmpTarget := snmp.Target{ID: deviceID, Address: target.Address, Port: target.SNMPPort, Credentials: target.SNMP, Timeout: target.Timeout, Retries: 1}
	res, derr := c.collector.Discover(ctx, snmpTarget)
	c.results[deviceID] = res
	c.errs[deviceID] = derr
	return res, derr
}

// operUp finds the named interface (case-insensitive exact match on
// ifDescr/ifName, falling back to a substring match) and reports its
// ifOperStatus, or an error if the device couldn't be reached or the
// interface wasn't found.
func operUp(res snmp.DiscoveryResult, ifaceName string) (bool, error) {
	name := strings.ToLower(strings.TrimSpace(ifaceName))
	if name == "" {
		return false, fmt.Errorf("interface name is empty")
	}
	// exact match first
	for _, iface := range res.Interfaces {
		if strings.ToLower(iface.Description) == name {
			return iface.OperUp, nil
		}
	}
	// substring fallback (e.g. configured "eth1" vs device's "GigabitEthernet0/1 eth1")
	for _, iface := range res.Interfaces {
		if strings.Contains(strings.ToLower(iface.Description), name) {
			return iface.OperUp, nil
		}
	}
	return false, fmt.Errorf("interface %q not found on device", ifaceName)
}

func (p *Poller) pollOnce(ctx context.Context) {
	links, err := p.Links.ListAll(ctx)
	if err != nil {
		log.Printf("topolinks poller: list links: %v", err)
		return
	}
	cache := &discoveryCache{collector: p.Collector, devices: p.Devices, results: map[string]snmp.DiscoveryResult{}, errs: map[string]error{}}
	now := time.Now().UTC()
	samples := make([]metricsdb.Sample, 0, len(links)*2)

	for _, link := range links {
		status := LinkStatus{LinkID: link.ID, CheckedAt: now}

		resA, errA := cache.get(ctx, link.DeviceAID)
		var upA, upB bool
		if errA != nil {
			status.Error = "device A: " + errA.Error()
		} else if up, err := operUp(resA, link.InterfaceA); err != nil {
			status.Error = "interface A: " + err.Error()
		} else {
			upA = up
			status.SideAUp = &up
		}

		resB, errB := cache.get(ctx, link.DeviceBID)
		if errB != nil {
			if status.Error != "" {
				status.Error += "; "
			}
			status.Error += "device B: " + errB.Error()
		} else if up, err := operUp(resB, link.InterfaceB); err != nil {
			if status.Error != "" {
				status.Error += "; "
			}
			status.Error += "interface B: " + err.Error()
		} else {
			upB = up
			status.SideBUp = &up
		}

		status.Up = status.SideAUp != nil && status.SideBUp != nil && upA && upB

		p.mu.Lock()
		p.live[link.ID] = status
		p.mu.Unlock()

		upVal := 0.0
		if status.Up {
			upVal = 1
		}
		samples = append(samples, metricsdb.Sample{SubjectType: "topology_link", SubjectID: link.ID, MetricName: "port_up", Value: upVal, RecordedAt: now})
		if status.SideAUp != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "device", SubjectID: link.DeviceAID, MetricName: "if_" + sanitizeMetricSuffix(link.InterfaceA) + "_up", Value: boolFloat(*status.SideAUp), RecordedAt: now})
		}
		if status.SideBUp != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "device", SubjectID: link.DeviceBID, MetricName: "if_" + sanitizeMetricSuffix(link.InterfaceB) + "_up", Value: boolFloat(*status.SideBUp), RecordedAt: now})
		}
	}

	if err := p.Metrics.RecordBatch(ctx, samples); err != nil {
		log.Printf("topolinks poller: record samples: %v", err)
	}
}

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// sanitizeMetricSuffix keeps metric_name readable/stable for free-text
// interface names (e.g. "eth1", "Gi0/1") by lowercasing and replacing
// anything that isn't alphanumeric.
func sanitizeMetricSuffix(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "unknown"
	}
	return string(out)
}

// Live returns the most recent status for a link id.
func (p *Poller) Live(linkID string) (LinkStatus, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.live[linkID]
	return s, ok
}

// LiveForGroup returns the most recent status for every link the caller
// passes in (typically all links in one group), keyed by link id.
func (p *Poller) LiveForGroup(linkIDs []string) map[string]LinkStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]LinkStatus, len(linkIDs))
	for _, id := range linkIDs {
		if s, ok := p.live[id]; ok {
			out[id] = s
		}
	}
	return out
}
