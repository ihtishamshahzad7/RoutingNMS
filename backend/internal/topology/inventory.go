package topology

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LiveGraph returns a Graph func (the shape API.Graph expects) built from
// real registered inventory: devices (routers/switches/servers, from the
// devices table) and OLTs (from the olts table). There is no scheduled LLDP
// discovery loop wired up yet, so links are intentionally left empty rather
// than invented — an accurate "no discovered links yet" is preferable to
// fabricated topology.
func LiveGraph(db *pgxpool.Pool) func() Graph {
	return func() Graph {
		if db == nil {
			return Builder{}.Build(nil, nil)
		}
		ctx := context.Background()
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

		return Builder{}.Build(nodes, nil)
	}
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
