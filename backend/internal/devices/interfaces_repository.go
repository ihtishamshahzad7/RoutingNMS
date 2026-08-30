package devices

import (
	"context"
	"fmt"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SaveInterfaces upserts discovered IF-MIB inventory. Re-running discovery is
// therefore safe and does not create duplicate interfaces.
func SaveInterfaces(ctx context.Context, db *pgxpool.Pool, deviceID string, interfaces []snmp.Interface) error {
	if db == nil { return fmt.Errorf("database is not initialized") }
	if deviceID == "" { return fmt.Errorf("device ID is required") }
	return saveInterfaces(ctx, db, deviceID, interfaces)
}

func saveInterfaces(ctx context.Context, db *pgxpool.Pool, deviceID string, items []snmp.Interface) error {
	for _, item := range items {
		var idx int64
		_, err := fmt.Sscan(item.Index, &idx)
		if err != nil { return fmt.Errorf("invalid interface index %q: %w", item.Index, err) }
		_, err = db.Exec(ctx, `INSERT INTO interfaces (device_id,if_index,name,description,admin_up,oper_up) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (device_id,if_index) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,admin_up=EXCLUDED.admin_up,oper_up=EXCLUDED.oper_up`, deviceID, idx, item.Description, item.Description, item.AdminUp, item.OperUp)
		if err != nil { return fmt.Errorf("save interface %s: %w", item.Index, err) }
	}
	return nil
}
