package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Repository persists discovered topology links (`topology_links`) and
// point-in-time graph snapshots (`topology_snapshots`), added by migration
// 0014. It is the data layer behind Sprint 1's scheduled LLDP discovery loop.
type Repository struct {
	DB *pgxpool.Pool
}

// PersistedLink is a `topology_links` row joined with device names so the API
// can render a graph without the frontend knowing device ids.
type PersistedLink struct {
	SourceID     int64     `json:"sourceId"`
	SourceName   string    `json:"sourceName"`
	SrcInterface string    `json:"srcInterface"`
	TargetID     int64     `json:"targetId"`
	TargetName   string    `json:"targetName"`
	DstInterface string    `json:"dstInterface"`
	LinkType     string    `json:"linkType"`
	IsActive     bool      `json:"isActive"`
	LatencyMs    float64   `json:"latencyMs,omitempty"`
	PacketLoss   float64   `json:"packetLossPct,omitempty"`
	LastVerified time.Time `json:"lastVerifiedAt"`
}

// DiscoveredLink is the engine's normalized output before it is persisted.
type DiscoveredLink struct {
	SourceID     int64
	SrcInterface string
	TargetID     int64
	DstInterface string
	LinkType     string
	LatencyMs    float64
	PacketLoss   float64
}

// LLDPSource is an SNMP-enabled device the discovery loop can walk the
// LLDP-MIB on, plus its name used to resolve remote neighbors back to a
// known device id.
type LLDPSource struct {
	ID       int64
	Name     string
	Address  string
	SNMP     snmp.Credentials
	SNMPPort uint16
	Timeout  time.Duration
}

// LLDPSources returns every enabled device that has SNMP monitoring enabled
// and is therefore a valid LLDP discovery target. Mirrors the ping poller's
// ListIcmpEnabled pattern.
func (r Repository) LLDPSources(ctx context.Context) ([]LLDPSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("topology repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,name,address,
		COALESCE(snmp_version,''),
		COALESCE(snmp_community,''),
		COALESCE(snmp_username,''),
		COALESCE(snmp_auth_protocol,''),
		COALESCE(snmp_auth_password,''),
		COALESCE(snmp_priv_protocol,''),
		COALESCE(snmp_priv_password,''),
		COALESCE(snmp_port,161),
		COALESCE(snmp_timeout_ms,3000)
		FROM devices WHERE enabled=true AND snmp_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LLDPSource{}
	for rows.Next() {
		var s LLDPSource
		var version string
		var timeoutMS int
		if err := rows.Scan(&s.ID, &s.Name, &s.Address,
			&version, &s.SNMP.Community, &s.SNMP.Username,
			&s.SNMP.AuthProto, &s.SNMP.AuthPass,
			&s.SNMP.PrivProto, &s.SNMP.PrivPass,
			&s.SNMPPort, &timeoutMS); err != nil {
			return nil, err
		}
		s.SNMP.Version = snmp.Version(version)
		s.Timeout = time.Duration(timeoutMS) * time.Millisecond
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReplaceActiveLinks persists the freshly discovered LLDP links for one
// discovery cycle. Each discovered link is upserted as active (touching
// last_verified_at); an LLDP link between two walked devices that is active
// but was not re-reported this cycle is marked inactive. By only deactivating
// links whose BOTH endpoints were walked this cycle, a single temporarily
// unreachable device does not wipe out links it was merely unable to report
// on this pass. Non-LLDP links (manual, cdp, ospf, bgp) are left untouched so
// future manual links survive rediscovery.
func (r Repository) ReplaceActiveLinks(ctx context.Context, links []DiscoveredLink) error {
	if r.DB == nil {
		return fmt.Errorf("topology repository is not initialized")
	}
	now := time.Now().UTC()
	for _, l := range links {
		if _, err := r.DB.Exec(ctx, `INSERT INTO topology_links
			(src_device_id, src_interface, dst_device_id, dst_interface, link_type, latency_ms, is_active, last_verified_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,TRUE,$7,$7)
			ON CONFLICT (src_device_id, src_interface, dst_device_id, dst_interface)
			DO UPDATE SET is_active=TRUE, last_verified_at=$7, updated_at=$7,
				link_type=EXCLUDED.link_type,
				latency_ms=COALESCE(EXCLUDED.latency_ms, topology_links.latency_ms)`,
			l.SourceID, l.SrcInterface, l.TargetID, l.DstInterface,
			l.LinkType, l.LatencyMs, now); err != nil {
			return fmt.Errorf("upsert link %d->%d: %w", l.SourceID, l.TargetID, err)
		}
	}
	return r.deactivateStale(ctx, links, now)
}

// deactivateStale marks active LLDP links whose both endpoints were walked
// this cycle but which were not re-reported as inactive.
func (r Repository) deactivateStale(ctx context.Context, links []DiscoveredLink, now time.Time) error {
	if len(links) == 0 {
		return nil
	}
	reported := map[string]bool{}
	walked := map[int64]bool{}
	for _, l := range links {
		reported[linkKey(l.SourceID, l.SrcInterface, l.TargetID, l.DstInterface)] = true
		walked[l.SourceID] = true
		walked[l.TargetID] = true
	}
	rows, err := r.DB.Query(ctx, `SELECT id,src_device_id,src_interface,dst_device_id,dst_interface
		FROM topology_links WHERE is_active AND link_type='lldp'`)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id, src, dst int64
		var sif, dif string
		if err := rows.Scan(&id, &src, &sif, &dst, &dif); err != nil {
			rows.Close()
			return err
		}
		// Only stale out if BOTH endpoints were walked this cycle and the
		// exact (src,if,dst,if) pair was not reported.
		if walked[src] && walked[dst] && !reported[linkKey(src, sif, dst, dif)] {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := r.DB.Exec(ctx, `UPDATE topology_links SET is_active=FALSE, updated_at=$2 WHERE id=$1`, id, now); err != nil {
			return err
		}
	}
	return nil
}

func linkKey(src int64, sif string, dst int64, dif string) string {
	return fmt.Sprintf("%d/%s|%d/%s", src, sif, dst, dif)
}

// ListActiveLinks returns all active persisted links joined with endpoint
// names, newest-verified first.
func (r Repository) ListActiveLinks(ctx context.Context) ([]PersistedLink, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("topology repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT l.src_device_id, s.name, l.src_interface,
			l.dst_device_id, d.name, l.dst_interface, l.link_type, l.is_active,
			COALESCE(l.latency_ms,0), l.last_verified_at
		FROM topology_links l
		JOIN devices s ON s.id=l.src_device_id
		JOIN devices d ON d.id=l.dst_device_id
		WHERE l.is_active
		ORDER BY l.last_verified_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersistedLink{}
	for rows.Next() {
		var p PersistedLink
		var lastVerified *time.Time
		if err := rows.Scan(&p.SourceID, &p.SourceName, &p.SrcInterface,
			&p.TargetID, &p.TargetName, &p.DstInterface, &p.LinkType, &p.IsActive,
			&p.LatencyMs, &lastVerified); err != nil {
			return nil, err
		}
		if lastVerified != nil {
			p.LastVerified = *lastVerified
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Snapshot is a stored point-in-time copy of the whole graph.
type Snapshot struct {
	ID         int64     `json:"id"`
	CapturedAt time.Time `json:"capturedAt"`
	NodeCount  int       `json:"nodeCount"`
	EdgeCount  int       `json:"edgeCount"`
	Graph      Graph     `json:"graph"`
}

// StoreSnapshot serializes a graph into the snapshots table and prunes
// anything older than 48h in the same transaction-ish flow.
func (r Repository) StoreSnapshot(ctx context.Context, g Graph) error {
	if r.DB == nil {
		return fmt.Errorf("topology repository is not initialized")
	}
	// Serialize as {nodes,edges} to keep payload size sane and match the
	// original prompt's graph vocabulary.
	payload := struct {
		Nodes []Node `json:"nodes"`
		Edges []Link `json:"edges"`
	}{Nodes: g.Nodes, Edges: g.Links}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := r.DB.Exec(ctx, `INSERT INTO topology_snapshots (captured_at, node_count, edge_count, graph_json)
		VALUES ($1,$2,$3,$4)`, g.GeneratedAt, len(g.Nodes), len(g.Links), raw); err != nil {
		return err
	}
	return r.PruneSnapshotsOlderThan(ctx, 48*time.Hour)
}

// ListSnapshots returns the `limit` most recent snapshots, newest first.
func (r Repository) ListSnapshots(ctx context.Context, limit int) ([]Snapshot, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("topology repository is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 24
	}
	rows, err := r.DB.Query(ctx, `SELECT id, captured_at, node_count, edge_count, graph_json
		FROM topology_snapshots ORDER BY captured_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		var s Snapshot
		var raw []byte
		if err := rows.Scan(&s.ID, &s.CapturedAt, &s.NodeCount, &s.EdgeCount, &raw); err != nil {
			return nil, err
		}
		var payload struct {
			Nodes []Node `json:"nodes"`
			Edges []Link `json:"edges"`
		}
		if err := json.Unmarshal(raw, &payload); err == nil {
			s.Graph = Graph{Nodes: payload.Nodes, Links: payload.Edges}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PruneSnapshotsOlderThan deletes snapshots older than age (48h default).
func (r Repository) PruneSnapshotsOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("topology repository is not initialized")
	}
	tag, err := r.DB.Exec(ctx, `DELETE FROM topology_snapshots WHERE captured_at < $1`, time.Now().Add(-age))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
