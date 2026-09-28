package devices

import (
	"context"
	"fmt"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SaveInterfaces upserts the latest IF-MIB inventory. Re-running discovery is
// safe and updates counters/status instead of creating duplicates.
func SaveInterfaces(ctx context.Context, db *pgxpool.Pool, deviceID string, interfaces []snmp.Interface) error {
	if db == nil { return fmt.Errorf("database is not initialized") }
	if deviceID == "" { return fmt.Errorf("device ID is required") }
	for _, item := range interfaces {
		var idx int64
		if _, err := fmt.Sscan(item.Index, &idx); err != nil { return fmt.Errorf("invalid interface index %q: %w", item.Index, err) }
		_, err := db.Exec(ctx, `INSERT INTO interfaces (device_id,if_index,name,description,admin_up,oper_up,in_octets,out_octets,in_errors,out_errors,last_discovered_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW()) ON CONFLICT (device_id,if_index) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,admin_up=EXCLUDED.admin_up,oper_up=EXCLUDED.oper_up,in_octets=EXCLUDED.in_octets,out_octets=EXCLUDED.out_octets,in_errors=EXCLUDED.in_errors,out_errors=EXCLUDED.out_errors,last_discovered_at=NOW()`, deviceID, idx, item.Description, item.Description, item.AdminUp, item.OperUp, item.InOctets, item.OutOctets, item.InErrors, item.OutErrors)
		if err != nil { return fmt.Errorf("save interface %s: %w", item.Index, err) }
	}
	return nil
}

type InterfaceRecord struct {
	ID int64 `json:"id"`
	DeviceID string `json:"deviceId"`
	IfIndex int64 `json:"ifIndex"`
	Name string `json:"name"`
	Description string `json:"description"`
	AdminUp bool `json:"adminUp"`
	OperUp bool `json:"operUp"`
	InOctets uint64 `json:"inOctets"`
	OutOctets uint64 `json:"outOctets"`
	InErrors uint64 `json:"inErrors"`
	OutErrors uint64 `json:"outErrors"`
	LastDiscoveredAt string `json:"lastDiscoveredAt,omitempty"`
	// Item 3.1 (SNMP interface polling) additions. These are additive-only
	// fields on the existing per-row shape -- the top-level response stays a
	// flat array, since frontend/app/(noc)/devices/[id]/page.tsx already
	// consumes GET /devices/{id}/interfaces expecting a bare Interface[].
	SpeedBps          uint64  `json:"speedBps"`
	CounterWidth      int     `json:"counterWidth"`
	InRateBps         *float64 `json:"inRateBps,omitempty"`
	OutRateBps        *float64 `json:"outRateBps,omitempty"`
	LastTransitionAt  string  `json:"lastTransitionAt,omitempty"`
	// Device-level SNMP reachability, duplicated onto every port row so a
	// consumer of this single endpoint can tell "SNMP unreachable" apart
	// from "port down" without a second request. Unknown is not the same
	// as down: when Reachable is false, AdminUp/OperUp above are the
	// last-known state, not a live read.
	SNMPReachable     bool    `json:"snmpReachable"`
	SNMPLastSuccessAt string  `json:"snmpLastSuccessAt,omitempty"`
	SNMPLastError     string  `json:"snmpLastError,omitempty"`
}

func (r Repository) ListInterfaces(ctx context.Context, deviceID string) ([]InterfaceRecord, error) {
	if r.DB == nil { return nil, fmt.Errorf("device repository is not initialized") }
	rows, err := r.DB.Query(ctx, `SELECT
			i.id,i.device_id,i.if_index,i.name,i.description,i.admin_up,i.oper_up,i.in_octets,i.out_octets,i.in_errors,i.out_errors,
			COALESCE(i.last_discovered_at::text,''),
			COALESCE(i.if_speed_bps,0),COALESCE(i.counter_width,64),i.in_rate_bps,i.out_rate_bps,COALESCE(i.last_transition_at::text,''),
			COALESCE(s.reachable,true),COALESCE(s.last_success_at::text,''),COALESCE(s.last_error,'')
		FROM interfaces i
		LEFT JOIN device_snmp_poll_state s ON s.device_id = i.device_id
		WHERE i.device_id=$1 ORDER BY i.if_index`, deviceID)
	if err != nil { return nil, err }
	defer rows.Close()
	items := []InterfaceRecord{}
	for rows.Next() {
		var item InterfaceRecord
		if err := rows.Scan(&item.ID,&item.DeviceID,&item.IfIndex,&item.Name,&item.Description,&item.AdminUp,&item.OperUp,&item.InOctets,&item.OutOctets,&item.InErrors,&item.OutErrors,
			&item.LastDiscoveredAt,
			&item.SpeedBps,&item.CounterWidth,&item.InRateBps,&item.OutRateBps,&item.LastTransitionAt,
			&item.SNMPReachable,&item.SNMPLastSuccessAt,&item.SNMPLastError); err != nil { return nil, err }
		items = append(items,item)
	}
	return items, rows.Err()
}
