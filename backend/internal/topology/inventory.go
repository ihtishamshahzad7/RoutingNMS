package topology

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LiveStatus is the minimal read accessor the graph builder needs to reflect
// a device's real, currently-known reachability in Node.Health. It's
// satisfied by a small adapter around the ICMP poller singleton wired up in
// main.go (the same singleton badges.Handler.PingPoller already reuses for
// its own "real state, don't invent it" status checks) — kept as an
// interface here so this package doesn't need to import internal/ping.
type LiveStatus interface {
	// Live reports whether deviceID's most recent probe was reachable, and
	// whether any probe result is known at all for it (ok=false for a
	// device this accessor has never probed, e.g. ICMP disabled) — callers
	// must treat ok=false as "unknown," not as down.
	Live(deviceID string) (reachable bool, ok bool)
}

// loadInventoryNodes returns the real registered inventory as graph nodes:
// devices (routers/switches/servers, from the devices table) and OLTs (from
// the olts table). Health defaults to 100 (rendered "up") for every node;
// when live is non-nil and has a known reading for a device, that reading
// overrides the default instead of leaving every node's status invented.
// OLTs aren't probed by the ICMP poller, so they always keep the default —
// same scope boundary as feature 31's badge pending-state closure, which
// only ever covered ICMP-monitored devices.
func loadInventoryNodes(ctx context.Context, db *pgxpool.Pool, live LiveStatus) []Node {
	if db == nil {
		return nil
	}
	nodes := []Node{}
	if rows, err := db.Query(ctx, `SELECT id, name, device_type, address FROM devices ORDER BY name`); err == nil {
		for rows.Next() {
			var id, name, deviceType, address string
			if rows.Scan(&id, &name, &deviceType, &address) == nil {
				health := 100
				if live != nil {
					if reachable, ok := live.Live(id); ok && !reachable {
						health = 0
					}
				}
				nodes = append(nodes, Node{ID: id, Name: name, Type: nodeType(deviceType), Address: address, Health: health})
			}
		}
		rows.Close()
	}
	if rows, err := db.Query(ctx, `SELECT id, name, address FROM olts ORDER BY name`); err == nil {
		for rows.Next() {
			var id, name, address string
			if rows.Scan(&id, &name, &address) == nil {
				nodes = append(nodes, Node{ID: id, Name: name, Type: OLT, Address: address, Health: 100})
			}
		}
		rows.Close()
	}
	return nodes
}

// LiveGraph returns a Graph func (the shape API.Graph expects) built from
// real registered inventory: devices (routers/switches/servers, from the
// devices table) and OLTs (from the olts table). If no discovery loop has
// been wired up, links are intentionally left empty rather than invented.
func LiveGraph(db *pgxpool.Pool) func() Graph {
	return func() Graph {
		return Builder{}.Build(loadInventoryNodes(context.Background(), db, nil), nil)
	}
}

// Graph builds the full persisted topology graph: inventory nodes plus active
// links from the topology_links table (endpoint ids are the devices.id as a
// decimal string, matching how loadInventoryNodes renders Node.ID).
func (r Repository) Graph(ctx context.Context) (Graph, error) {
	nodes := loadInventoryNodes(ctx, r.DB, r.Live)
	links, err := r.ListActiveLinks(ctx)
	if err != nil {
		return Graph{}, err
	}
	// Map node id (string device id) -> Node so we can resolve link endpoints
	// and keep a consistent set.
	byID := map[string]Node{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	relationships := make([]Relationship, 0, len(links))
	for _, l := range links {
		src := fmt.Sprint(l.SourceID)
		dst := fmt.Sprint(l.TargetID)
		if _, ok := byID[src]; !ok {
			continue
		}
		if _, ok := byID[dst]; !ok {
			continue
		}
		st := Up
		if !l.IsActive {
			st = Down
		}
		relationships = append(relationships, Relationship{
			SourceID: src, TargetID: dst, Status: st,
			LatencyMs: l.LatencyMs, PacketLossPct: l.PacketLoss,
		})
	}
	g := Builder{}.Build(nodes, relationships)
	return g, nil
}

func nodeType(deviceType string) NodeType {
	switch deviceType {
	case "router":
		return Router
	case "switch":
		return Switch
	case "olt":
		return OLT
	case "server":
		return Server
	default:
		return Unknown
	}
}
