package topology

import "time"

type NodeType string
const (
	Router NodeType = "router"
	Switch NodeType = "switch"
	OLT NodeType = "olt"
	ONU NodeType = "onu"
	Server NodeType = "server"
	Unknown NodeType = "unknown"
)

type LinkStatus string
const (
	Up LinkStatus = "up"
	Down LinkStatus = "down"
	Degraded LinkStatus = "degraded"
	UnknownLink LinkStatus = "unknown"
)

type Node struct { ID string `json:"id"`; Name string `json:"name"`; Type NodeType `json:"type"`; Address string `json:"address,omitempty"`; Health int `json:"health"` }
type Link struct { ID string `json:"id"`; Source string `json:"source"`; Target string `json:"target"`; Status LinkStatus `json:"status"`; LatencyMs float64 `json:"latencyMs,omitempty"`; PacketLossPct float64 `json:"packetLossPct,omitempty"` }
type Graph struct { Nodes []Node `json:"nodes"`; Links []Link `json:"links"`; GeneratedAt time.Time `json:"generatedAt"` }
