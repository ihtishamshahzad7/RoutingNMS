package topology

import "time"

// Relationship describes a discovered parent/child connection. Discovery
// adapters can produce these from LLDP, CDP, OLT topology, or configuration.
type Relationship struct {
	SourceID string
	TargetID string
	Status LinkStatus
	LatencyMs float64
	PacketLossPct float64
}

type Builder struct{}

func (Builder) Build(nodes []Node, relationships []Relationship) Graph {
	links := make([]Link, 0, len(relationships))
	for _, r := range relationships {
		links = append(links, Link{ID:r.SourceID+"->"+r.TargetID, Source:r.SourceID, Target:r.TargetID, Status:r.Status, LatencyMs:r.LatencyMs, PacketLossPct:r.PacketLossPct})
	}
	return Graph{Nodes:nodes, Links:links, GeneratedAt:time.Now().UTC()}
}
