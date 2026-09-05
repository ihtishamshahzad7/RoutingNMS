// Package topolinks implements group-wise, port-level topology link
// mapping: an operator explicitly states "device A interface ethX is
// connected to device B interface ethY", organized into named groups, and
// this package polls each end's SNMP ifOperStatus and reports up/down
// through the same metricsdb + alertsfeed machinery as every other monitor
// type in this codebase. This is intentionally separate from
// internal/topology, which auto-discovers a graph via LLDP and has no
// concept of an operator-defined link or group.
package topolinks

import "time"

// Group is a named collection of topology links, e.g. "Group 1" /
// "Core Ring" -- how an operator organizes related port mappings.
type Group struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Link is one manually-defined bidirectional connection: device A's
// interface A is connected to device B's interface B. DeviceAName/
// DeviceBName are joined in for display, not stored on the row itself.
type Link struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"groupId"`
	DeviceAID   string    `json:"deviceAId"`
	DeviceAName string    `json:"deviceAName,omitempty"`
	InterfaceA  string    `json:"interfaceA"`
	DeviceBID   string    `json:"deviceBId"`
	DeviceBName string    `json:"deviceBName,omitempty"`
	InterfaceB  string    `json:"interfaceB"`
	CreatedAt   time.Time `json:"createdAt"`
}

// LinkStatus is the live up/down state of one link's two ends, as returned
// by the live-status endpoint and recorded as metric samples.
type LinkStatus struct {
	LinkID    string    `json:"linkId"`
	Up        bool      `json:"up"` // true only if both ends report ifOperStatus=up
	SideAUp   *bool     `json:"sideAUp"`
	SideBUp   *bool     `json:"sideBUp"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
}
