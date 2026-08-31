// Package metricsdb implements simple Postgres-backed time-series storage
// for per-device / per-OLT / per-ONU metric history (feature 3 of the
// roadmap: "per-device metric history / charts"). It deliberately reuses
// this project's existing Postgres database rather than depending on the
// VictoriaMetrics container referenced in deployments/docker-compose.yml,
// which isn't part of the actual production deployment path
// (deployments/ubuntu-24.04/update.sh runs bare systemd services, no
// Docker) -- so charts work with zero extra infrastructure to stand up.
package metricsdb

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

// Sample is one metric value to persist.
type Sample struct {
	SubjectType string
	SubjectID   string
	MetricName  string
	Value       float64
	RecordedAt  time.Time
}

// Point is one value in a returned time series.
type Point struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Series is one metric's history for one subject.
type Series struct {
	Metric string  `json:"metric"`
	Points []Point `json:"points"`
}

// RecordBatch persists many samples inside one transaction. Kept as a
// straightforward per-row Exec loop (rather than pgx's separate batch API)
// to match the transaction pattern already used elsewhere in this codebase
// (see internal/mib.Repository.Upload) that's known to work against this
// project's pgxpool version.
func (r Repository) RecordBatch(ctx context.Context, samples []Sample) error {
	if r.DB == nil {
		return fmt.Errorf("metricsdb repository is not initialized")
	}
	if len(samples) == 0 {
		return nil
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, s := range samples {
		ts := s.RecordedAt
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		if _, err := tx.Exec(ctx, `INSERT INTO metric_samples (subject_type,subject_id,metric_name,value,recorded_at) VALUES ($1,$2,$3,$4,$5)`,
			s.SubjectType, s.SubjectID, s.MetricName, s.Value, ts); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Query returns each requested metric's history for one subject over the
// last `since` duration, ordered oldest-first (chart-ready).
func (r Repository) Query(ctx context.Context, subjectType, subjectID string, metricNames []string, since time.Duration) ([]Series, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("metricsdb repository is not initialized")
	}
	out := make([]Series, 0, len(metricNames))
	cutoff := time.Now().UTC().Add(-since)
	for _, metric := range metricNames {
		rows, err := r.DB.Query(ctx, `SELECT value, recorded_at FROM metric_samples
			WHERE subject_type=$1 AND subject_id=$2 AND metric_name=$3 AND recorded_at >= $4
			ORDER BY recorded_at ASC`, subjectType, subjectID, metric, cutoff)
		if err != nil {
			return nil, err
		}
		series := Series{Metric: metric, Points: []Point{}}
		for rows.Next() {
			var p Point
			if err := rows.Scan(&p.Value, &p.Timestamp); err != nil {
				rows.Close()
				return nil, err
			}
			series.Points = append(series.Points, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		out = append(out, series)
	}
	return out, nil
}

// PruneOlderThan deletes samples older than age, mirroring the retention
// pattern used by the syslog and snmptrap packages.
func (r Repository) PruneOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("metricsdb repository is not initialized")
	}
	tag, err := r.DB.Exec(ctx, `DELETE FROM metric_samples WHERE recorded_at < $1`, time.Now().Add(-age))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
