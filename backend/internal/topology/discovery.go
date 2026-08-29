package topology

import "context"

type Neighbor struct { LocalID string; RemoteID string; LatencyMs float64; PacketLossPct float64; Status LinkStatus }

type NeighborDiscovery interface { Discover(ctx context.Context, node Node) ([]Neighbor, error) }

// DiscoverGraph converts adapter-specific neighbor information into the
// normalized topology graph. Adapters can use LLDP, CDP, OLT relationships,
// or configured links without changing the graph model.
func DiscoverGraph(ctx context.Context, nodes []Node, discovery NeighborDiscovery) (Graph, error) {
	relationships := make([]Relationship,0)
	for _, node := range nodes {
		neighbors, err := discovery.Discover(ctx,node); if err != nil { return Graph{}, err }
		for _, n := range neighbors { relationships=append(relationships,Relationship{SourceID:n.LocalID,TargetID:n.RemoteID,Status:n.Status,LatencyMs:n.LatencyMs,PacketLossPct:n.PacketLossPct}) }
	}
	return Builder{}.Build(nodes,relationships),nil
}
