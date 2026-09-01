package topology

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// loadInventoryNodes returns the real registered inventory as graph nodes:
// devices (routers/switches/servers, from the devices table) and OLTs (from
// the olts table), each given full health so a page load shows them up.
func loadInventoryNodes(ctx context.Context, db *pgxpool.Pool) []Node {
	if db == nil {
		return nil
	}
	nodes := []Node{}
	if rows, err := db.Query(ctx, `SELECT id, name, device_type, address FROM devices ORDER BY name`); err == nil {
		for rows.Next() {
			var id, name, deviceType, address string
			if rows.Scan(&id, &name, &deviceType, &address) == nil {
				nodes = append(nodes, Node{ID: id, Name: name, Type: nodeType(deviceType), Address: address, Health: 100})
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
		return Builder{}.Build(loadInventoryNodes(context.Background(), db), nil)
	}
}

// Graph builds the full persisted topology graph: inventory nodes plus active
// links from the topology_links table (endpoint ids are the devices.id as a
// decimal string, matching how loadInventoryNodes renders Node.ID).
func (r Repository) Graph(ctx context.Context) (Graph, error) {
	nodes := loadInventoryNodes(ctx, r.DB)
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
