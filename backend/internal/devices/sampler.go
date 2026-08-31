package devices

import (
	"context"
	"log"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
)

// SamplePeriodically probes every enabled device (reusing the same
// SNMP-connect/TCP-ping health check the on-demand /devices/health
// endpoint uses) on an interval and records "up" and "latency_ms" samples
// per device, powering the per-device metric history charts.
func SamplePeriodically(ctx context.Context, repo Repository, metrics metricsdb.Repository, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	sampleOnce(ctx, repo, metrics)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sampleOnce(ctx, repo, metrics)
		}
	}
}

func sampleOnce(ctx context.Context, repo Repository, metrics metricsdb.Repository) {
	devices, err := repo.ListAllEnabled(ctx)
	if err != nil {
		log.Printf("device metric sampler: list devices: %v", err)
		return
	}
	if len(devices) == 0 {
		return
	}
	now := time.Now().UTC()
	samples := make([]metricsdb.Sample, 0, len(devices)*2)
	for _, d := range devices {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		health := ProbeDevice(probeCtx, repo, d)
		cancel()
		up := 0.0
		if health.Reachable {
			up = 1
		}
		samples = append(samples,
			metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, MetricName: "up", Value: up, RecordedAt: now},
			metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, MetricName: "latency_ms", Value: health.LatencyMS, RecordedAt: now},
		)
	}
	if err := metrics.RecordBatch(ctx, samples); err != nil {
		log.Printf("device metric sampler: record batch: %v", err)
	}
}
