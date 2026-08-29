package olt

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct { DB *pgxpool.Pool }

func (r Repository) UpsertPON(ctx context.Context, oltID string, p PONPort) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	_, err := r.DB.Exec(ctx, `INSERT INTO olt_pons (id,olt_id,name,status,onu_count,last_seen) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,status=EXCLUDED.status,onu_count=EXCLUDED.onu_count,last_seen=EXCLUDED.last_seen,updated_at=NOW()`, p.ID, oltID, p.Name, p.Status, p.ONUCount, p.LastSeen)
	return err
}

func (r Repository) UpsertONU(ctx context.Context, oltID, ponID string, o ONU) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	_, err := r.DB.Exec(ctx, `INSERT INTO olt_onus (id,olt_id,pon_id,serial_number,name,status,los,rx_power_dbm,tx_power_dbm,distance_meters,last_seen) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT (id) DO UPDATE SET pon_id=EXCLUDED.pon_id,serial_number=EXCLUDED.serial_number,name=EXCLUDED.name,status=EXCLUDED.status,los=EXCLUDED.los,rx_power_dbm=EXCLUDED.rx_power_dbm,tx_power_dbm=EXCLUDED.tx_power_dbm,distance_meters=EXCLUDED.distance_meters,last_seen=EXCLUDED.last_seen,updated_at=NOW()`, o.ID, oltID, ponID, o.Serial, o.Name, o.Status, o.LOS, o.RxPowerDBm, o.TxPowerDBm, o.DistanceMeters, o.LastSeen)
	return err
}

func (r Repository) SavePollResult(ctx context.Context, oltID string, result PollResult) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	for _, p := range result.PONs { if err := r.UpsertPON(ctx, oltID, p); err != nil { return err } }
	for _, o := range result.ONUs { if err := r.UpsertONU(ctx, oltID, o.PONPortID, o); err != nil { return err } }
	return nil
}

type AlertRecord struct { ID int64; OLTID, PONID, ONUID, Code, Severity, Message, Status string; Value *float64; FirstSeen, LastSeen time.Time }

func (r Repository) UpsertAlert(ctx context.Context, oltID, ponID, onuID string, a OpticalAlert, at time.Time) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	_, err := r.DB.Exec(ctx, `INSERT INTO olt_alerts (olt_id,pon_id,onu_id,code,severity,message,value,last_seen) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (olt_id,COALESCE(onu_id,''),code) WHERE status='open' DO UPDATE SET pon_id=EXCLUDED.pon_id,severity=EXCLUDED.severity,message=EXCLUDED.message,value=EXCLUDED.value,last_seen=EXCLUDED.last_seen`, oltID, ponID, onuID, a.Code, a.Severity, a.Message, a.Value, at)
	return err
}

func (r Repository) ClearResolvedAlerts(ctx context.Context, oltID string, active map[string]bool, at time.Time) error {
	if r.DB == nil { return fmt.Errorf("database is not initialized") }
	rows, err := r.DB.Query(ctx, `SELECT id,onu_id,code FROM olt_alerts WHERE olt_id=$1 AND status='open'`, oltID); if err != nil{return err}; defer rows.Close()
	for rows.Next(){var id int64;var onu,code string;if err:=rows.Scan(&id,&onu,&code);err!=nil{return err};if !active[onu+":"+code]{if _,err:=r.DB.Exec(ctx,`UPDATE olt_alerts SET status='cleared',cleared_at=$2,last_seen=$2 WHERE id=$1`,id,at);err!=nil{return err}}}
	return rows.Err()
}
