package olt

import (
	"context"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// MetricSampler converts a PollResult into metricsdb samples for the
// per-device/OLT metric history feature. This is the Postgres-backed
// counterpart to MetricsWriter (which targets the VictoriaMetrics
// container that isn't part of the actual production deployment) --
// see internal/metricsdb's package doc for why.
type MetricSampler struct{ Repo metricsdb.Repository }

func (s MetricSampler) Record(ctx context.Context, oltID string, result PollResult) error {
	if s.Repo.DB == nil {
		return nil
	}
	now := result.PolledAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	samples := make([]metricsdb.Sample, 0, len(result.PONs)+len(result.ONUs)*3)
	for _, p := range result.PONs {
		up := 0.0
		if p.Status == Online {
			up = 1
		}
		samples = append(samples, metricsdb.Sample{SubjectType: "pon", SubjectID: p.ID, MetricName: "up", Value: up, RecordedAt: now})
		samples = append(samples, metricsdb.Sample{SubjectType: "pon", SubjectID: p.ID, MetricName: "onu_count", Value: float64(p.ONUCount), RecordedAt: now})
	}
	for _, o := range result.ONUs {
		if o.RxPowerDBm != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "onu", SubjectID: o.ID, MetricName: "rx_power_dbm", Value: *o.RxPowerDBm, RecordedAt: now})
		}
		if o.TxPowerDBm != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "onu", SubjectID: o.ID, MetricName: "tx_power_dbm", Value: *o.TxPowerDBm, RecordedAt: now})
		}
		if o.DistanceMeters != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "onu", SubjectID: o.ID, MetricName: "distance_meters", Value: *o.DistanceMeters, RecordedAt: now})
		}
	}
	if len(samples) == 0 {
		return nil
	}
	return s.Repo.RecordBatch(ctx, samples)
}
