package topology

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Discovery is the Sprint 1 scheduled LLDP discovery engine. It walks the
// LLDP-MIB of every SNMP-enabled device, resolves discovered remote neighbors
// back to known device ids by system name, persists the resulting links into
// topology_links, and records a point-in-time snapshot of the full graph.
//
// Before this engine, the topology package could model and walk LLDP but
// nothing scheduled it or persisted links — the topology page always showed
// zero links. This closes that gap.
type Discovery struct {
	Repo      Repository
	Collector snmp.Collector
	Interval  time.Duration

	mu       sync.Mutex
	lastRun  time.Time
	lastDur  time.Duration
	lastErr  string
	lastLink int
	lastNode int
	running  bool
}

// NewDiscovery builds a discovery engine. interval <= 0 defaults to 15m.
func NewDiscovery(repo Repository) *Discovery {
	return &Discovery{Repo: repo, Collector: snmp.Collector{}, Interval: 15 * time.Minute}
}

// Run starts the periodic discovery loop: one pass immediately, then every
// Interval. Modeled on ping.Poller.Run and devices.SamplePeriodically.
func (d *Discovery) Run(ctx context.Context) {
	ticker := time.NewTicker(d.ckInterval())
	defer ticker.Stop()
	d.discoverOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.discoverOnce(ctx)
		}
	}
}

func (d *Discovery) ckInterval() time.Duration {
	if d.Interval <= 0 {
		return 15 * time.Minute
	}
	return d.Interval
}

// Status returns the outcome of the most recent cycle, for the UI to show
// when the last discovery ran and whether it is currently running.
func (d *Discovery) Status() DiscoveryStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DiscoveryStatus{
		LastRun:   d.lastRun,
		Duration:  d.lastDur.Milliseconds(),
		LastError: d.lastErr,
		Links:     d.lastLink,
		Nodes:     d.lastNode,
		Running:   d.running,
		Interval:  d.ckInterval().Milliseconds(),
	}
}

// DiscoveryStatus is the public status snapshot exposed to the UI. Duration
// and Interval are expressed in milliseconds so they serialize as friendly
// JSON ints rather than raw time.Duration nanoseconds.
type DiscoveryStatus struct {
	LastRun   time.Time `json:"lastRun"`
	Duration  int64     `json:"durationMs"`
	LastError string    `json:"lastError,omitempty"`
	Links     int       `json:"links"`
	Nodes     int       `json:"nodes"`
	Running   bool      `json:"running"`
	Interval  int64     `json:"intervalMs"`
}

// DiscoverNow runs a single discovery cycle synchronously and returns the
// number of persisted links. It is safe to call from a request handler to
// trigger a manual rediscovery (POST /api/v1/topology/discover).
func (d *Discovery) DiscoverNow(ctx context.Context) (int, error) {
	return d.discoverOnce(ctx)
}

func (d *Discovery) discoverOnce(ctx context.Context) (int, error) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return d.lastLink, nil
	}
	d.running = true
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()

	started := time.Now().UTC()
	links, err := d.collect(ctx)

	d.mu.Lock()
	d.lastRun = started
	d.lastDur = time.Since(started)
	d.lastErr = ""
	d.lastLink = len(links)
	if err != nil {
		d.lastErr = err.Error()
		log.Printf("topology discovery: %v", err)
	}
	d.mu.Unlock()

	// Persist links + snapshot even on a partial error so a transient device
	// failure still advances the graph for the devices that responded.
	if len(links) > 0 && d.Repo.DB != nil {
		if err := d.Repo.ReplaceActiveLinks(ctx, links); err != nil {
			log.Printf("topology discovery: persist links: %v", err)
		}
	}
	if g, gerr := d.Repo.Graph(ctx); gerr == nil {
		d.lastNode = len(g.Nodes)
		if err := d.Repo.StoreSnapshot(ctx, g); err != nil {
			log.Printf("topology discovery: store snapshot: %v", err)
		}
	}
	return len(links), err
}

// collect walks the LLDP-MIB of every SNMP-enabled device (concurrently, with
// a per-device timeout) and resolves remote neighbors back to known device
// ids. Neighbors whose remote system name does not match a known device are
// skipped — we never persist an edge to an unknown device id (the FK requires
// a real devices.id).
func (d *Discovery) collect(ctx context.Context) ([]DiscoveredLink, error) {
	sources, err := d.Repo.LLDPSources(ctx)
	if err != nil {
		return nil, err
	}

	byName := map[string]int64{}
	for _, s := range sources {
		byName[keyName(s.Name)] = s.ID
	}

	var mu sync.Mutex
	links := []DiscoveredLink{}
	var wg sync.WaitGroup
	for _, s := range sources {
		wg.Add(1)
		go func(s LLDPSource) {
			defer wg.Done()
			// Resolve the SNMP target for this source once, then walk its
			// LLDP-MIB neighbors.
			target := snmp.Target{
				ID:          s.Name,
				Address:     s.Address,
				Port:        s.SNMPPort,
				Credentials: s.SNMP,
				Timeout:     s.Timeout,
				Retries:     1,
			}
			discovery := SNMPNeighborDiscovery{Collector: d.Collector, Resolve: func(Node) (snmp.Target, error) { return target, nil }}
			node := Node{ID: strconv.FormatInt(s.ID, 10), Name: s.Name, Address: s.Address, Health: 100}
			probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
			defer cancel()
			neighbors, derr := discovery.Discover(probeCtx, node)
			if derr != nil {
				return // not an error that should abort the whole cycle
			}
			for _, n := range neighbors {
				targetID, ok := byName[keyName(n.RemoteID)]
				if !ok {
					continue // remote is not a device we know; skip
				}
				mu.Lock()
				links = append(links, DiscoveredLink{
					SourceID: s.ID, TargetID: targetID,
					LinkType: "lldp",
				})
				mu.Unlock()
			}
		}(s)
	}
	wg.Wait()
	return links, nil
}

func keyName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
