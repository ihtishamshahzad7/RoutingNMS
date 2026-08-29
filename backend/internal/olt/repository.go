package olt

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct { DB *pgxpool.Pool }

func (r Repository) UpsertPON(ctx context.Context, oltID string, p PON) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	_, err := r.DB.Exec(ctx, `INSERT INTO olt_pons (id,olt_id,name,status,onu_count) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,status=EXCLUDED.status,onu_count=EXCLUDED.onu_count`, p.ID, oltID, p.Name, p.Status, p.ONUCount)
	return err
}

func (r Repository) UpsertONU(ctx context.Context, oltID string, ponID string, o ONU) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	_, err := r.DB.Exec(ctx, `INSERT INTO olt_onus (id,olt_id,pon_id,serial_number,name,status,los,rx_power_dbm,tx_power_dbm,distance_meters,last_seen) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT (id) DO UPDATE SET pon_id=EXCLUDED.pon_id,serial_number=EXCLUDED.serial_number,name=EXCLUDED.name,status=EXCLUDED.status,los=EXCLUDED.los,rx_power_dbm=EXCLUDED.rx_power_dbm,tx_power_dbm=EXCLUDED.tx_power_dbm,distance_meters=EXCLUDED.distance_meters,last_seen=EXCLUDED.last_seen`, o.ID, oltID, ponID, o.SerialNumber, o.Name, o.Status, o.LOS, o.RXPowerDBm, o.TXPowerDBm, o.DistanceMeters, o.LastSeen)
	return err
}

func (r Repository) SavePollResult(ctx context.Context, oltID string, result PollResult) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	for _, p := range result.PONs { if err := r.UpsertPON(ctx, oltID, p); err != nil { return err } }
	for _, o := range result.ONUs { if err := r.UpsertONU(ctx, oltID, "", o); err != nil { return err } }
	return nil
}
