package olt

import (
	"context"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metrics"
)

// MetricsWriter converts the normalized OLT model into time-series samples.
type MetricsWriter struct { Writer metrics.Writer }

func (w MetricsWriter) Write(ctx context.Context, oltID string, result PollResult) error {
	now := result.PolledAt
	if now.IsZero() { now = time.Now().UTC() }
	samples := make([]metrics.Sample, 0, len(result.PONs)*2+len(result.ONUs)*5)
	for _, p := range result.PONs {
		labels := map[string]string{"olt_id": oltID, "pon_id": p.ID, "pon": p.Name}
		up := 0.0; if p.Status == Online { up = 1 }
		samples = append(samples,
			metrics.Sample{Name:"olt_pon_up", Value:up, Timestamp:now, Labels:labels},
			metrics.Sample{Name:"olt_pon_onu_count", Value:float64(p.ONUCount), Timestamp:now, Labels:labels},
		)
	}
	for _, o := range result.ONUs {
		labels := map[string]string{"olt_id": oltID, "onu_id": o.ID, "serial": o.Serial}
		up := 0.0; if o.Status == Online { up = 1 }
		los := 0.0; if o.LOS { los = 1 }
		samples = append(samples,
			metrics.Sample{Name:"olt_onu_up", Value:up, Timestamp:now, Labels:labels},
			metrics.Sample{Name:"olt_onu_los", Value:los, Timestamp:now, Labels:labels},
		)
		if o.RxPowerDBm != nil { samples = append(samples, metrics.Sample{Name:"olt_onu_rx_power_dbm", Value:*o.RxPowerDBm, Timestamp:now, Labels:labels}) }
		if o.TxPowerDBm != nil { samples = append(samples, metrics.Sample{Name:"olt_onu_tx_power_dbm", Value:*o.TxPowerDBm, Timestamp:now, Labels:labels}) }
		if o.DistanceMeters != nil { samples = append(samples, metrics.Sample{Name:"olt_onu_distance_meters", Value:*o.DistanceMeters, Timestamp:now, Labels:labels}) }
	}
	return w.Writer.Write(ctx, samples)
}
