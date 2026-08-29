package olt

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
)

type AlertSummary struct { Open int `json:"open"`; Critical int `json:"critical"`; Warning int `json:"warning"`; Cleared24h int `json:"cleared_24h"` }

func GetAlertSummary(ctx context.Context, db *pgxpool.Pool, oltID string) (AlertSummary, error) {
    if db == nil { return AlertSummary{}, fmt.Errorf("database is not initialized") }
    var s AlertSummary
    err := db.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE status='open'), COUNT(*) FILTER (WHERE status='open' AND severity='critical'), COUNT(*) FILTER (WHERE status='open' AND severity='warning'), COUNT(*) FILTER (WHERE status='cleared' AND cleared_at >= NOW()-INTERVAL '24 hours') FROM olt_alerts WHERE olt_id=$1`, oltID).Scan(&s.Open,&s.Critical,&s.Warning,&s.Cleared24h)
    return s, err
}
