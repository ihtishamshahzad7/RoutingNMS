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

// Record persists PollResult as metric_samples, tagging every sample with
// tenantID (the owning OLT's organization_id) so per-tenant data retention
// (internal/retention) and any future per-tenant metric views actually
// cover OLT/PON/ONU data instead of it all landing in the shared ""
// bucket. tenantID is "" for an OLT that predates organization_id or was
// never assigned one, matching every other tenant-scoped subject in this
// codebase.
func (s MetricSampler) Record(ctx context.Context, oltID string, tenantID string, result PollResult) error {
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
		samples = append(samples, metricsdb.Sample{SubjectType: "pon", SubjectID: p.ID, MetricName: "up", Value: up, RecordedAt: now, TenantID: tenantID})
		samples = append(samples, metricsdb.Sample{SubjectType: "pon", SubjectID: p.ID, MetricName: "onu_count", Value: float64(p.ONUCount), RecordedAt: now, TenantID: tenantID})
	}
	for _, o := range result.ONUs {
		if o.RxPowerDBm != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "onu", SubjectID: o.ID, MetricName: "rx_power_dbm", Value: *o.RxPowerDBm, RecordedAt: now, TenantID: tenantID})
		}
		if o.TxPowerDBm != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "onu", SubjectID: o.ID, MetricName: "tx_power_dbm", Value: *o.TxPowerDBm, RecordedAt: now, TenantID: tenantID})
		}
		if o.DistanceMeters != nil {
			samples = append(samples, metricsdb.Sample{SubjectType: "onu", SubjectID: o.ID, MetricName: "distance_meters", Value: *o.DistanceMeters, RecordedAt: now, TenantID: tenantID})
		}
	}
	if len(samples) == 0 {
		return nil
	}
	return s.Repo.RecordBatch(ctx, samples)
}
