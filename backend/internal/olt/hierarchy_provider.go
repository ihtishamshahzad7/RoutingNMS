package olt

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBProvider implements Provider (see api.go) by reading the real OLT/PON/ONU
// inventory persisted by the poller (olts, olt_pons, olt_onus — see
// migrations/0001_olt_configuration.sql). It replaces PON.Port (an integer
// position expected by the frontend) with the row's position among that
// OLT's PONs, since olt_pons only stores a free-text name, not a numeric
// port.
type DBProvider struct{ DB *pgxpool.Pool }

func (p DBProvider) GetHierarchy(id string) (Hierarchy, bool) {
	if p.DB == nil {
		return Hierarchy{}, false
	}
	ctx := context.Background()
	var name, model string
	if err := p.DB.QueryRow(ctx, `SELECT name, model FROM olts WHERE id=$1`, id).Scan(&name, &model); err != nil {
		return Hierarchy{}, false
	}

	ponRows, err := p.DB.Query(ctx, `SELECT id, name, status FROM olt_pons WHERE olt_id=$1 ORDER BY name`, id)
	if err != nil {
		return Hierarchy{}, false
	}
	defer ponRows.Close()

	type ponRow struct{ id, name, status string }
	pons := []ponRow{}
	for ponRows.Next() {
		var pr ponRow
		if err := ponRows.Scan(&pr.id, &pr.name, &pr.status); err != nil {
			return Hierarchy{}, false
		}
		pons = append(pons, pr)
	}
	if err := ponRows.Err(); err != nil {
		return Hierarchy{}, false
	}

	onusByPON, err := p.onusByPON(ctx, id)
	if err != nil {
		return Hierarchy{}, false
	}

	out := make([]PON, 0, len(pons))
	for i, pr := range pons {
		out = append(out, PON{ID: pr.id, Port: i + 1, Status: Status(pr.status), ONUs: onusByPON[pr.id]})
	}
	return NewHierarchy(id, name, model, out), true
}

func (p DBProvider) onusByPON(ctx context.Context, oltID string) (map[string][]ONU, error) {
	rows, err := p.DB.Query(ctx, `SELECT id, pon_id, serial_number, name, status, los, rx_power_dbm, tx_power_dbm FROM olt_onus WHERE olt_id=$1 ORDER BY name`, oltID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]ONU{}
	for rows.Next() {
		var o ONU
		var rx, tx sql.NullFloat64
		if err := rows.Scan(&o.ID, &o.PONPortID, &o.Serial, &o.Name, &o.Status, &o.LOS, &rx, &tx); err != nil {
			return nil, err
		}
		if rx.Valid {
			f := rx.Float64
			o.RxPowerDBm = &f
		}
		if tx.Valid {
			f := tx.Float64
			o.TxPowerDBm = &f
		}
		out[o.PONPortID] = append(out[o.PONPortID], o)
	}
	return out, rows.Err()
}
