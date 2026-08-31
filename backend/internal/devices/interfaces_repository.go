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
}

func (r Repository) ListInterfaces(ctx context.Context, deviceID string) ([]InterfaceRecord, error) {
	if r.DB == nil { return nil, fmt.Errorf("device repository is not initialized") }
	rows, err := r.DB.Query(ctx, `SELECT id,device_id,if_index,name,description,admin_up,oper_up,in_octets,out_octets,in_errors,out_errors,COALESCE(last_discovered_at::text,'') FROM interfaces WHERE device_id=$1 ORDER BY if_index`, deviceID)
	if err != nil { return nil, err }
	defer rows.Close()
	items := []InterfaceRecord{}
	for rows.Next() {
		var item InterfaceRecord
		if err := rows.Scan(&item.ID,&item.DeviceID,&item.IfIndex,&item.Name,&item.Description,&item.AdminUp,&item.OperUp,&item.InOctets,&item.OutOctets,&item.InErrors,&item.OutErrors,&item.LastDiscoveredAt); err != nil { return nil, err }
		items = append(items,item)
	}
	return items, rows.Err()
}
