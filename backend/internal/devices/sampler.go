package devices

import (
	"context"
	"log"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/httpcheck"
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
			metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "up", Value: up, RecordedAt: now},
			metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "latency_ms", Value: health.LatencyMS, RecordedAt: now},
		)

		// Optional HTTP(S)+keyword monitor (ported from Uptime Kuma) --
		// independent of the SNMP/ICMP check above, so a device can be
		// "SNMP down" and "HTTP up" (or vice versa) at the same time.
		if d.HTTPCheckEnabled && d.HTTPURL != "" {
			httpCtx, httpCancel := context.WithTimeout(ctx, time.Duration(d.HTTPTimeoutMS)*time.Millisecond+time.Second)
			result := httpcheck.Check(httpCtx, d.HTTPURL, d.HTTPExpectedStatus, d.HTTPKeyword, time.Duration(d.HTTPTimeoutMS)*time.Millisecond)
			httpCancel()
			httpUp := 0.0
			if result.Reachable {
				httpUp = 1
			}
			samples = append(samples,
				metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "http_up", Value: httpUp, RecordedAt: now},
				metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "http_latency_ms", Value: result.LatencyMS, RecordedAt: now},
			)
			if result.CertExpiryInDays != nil {
				samples = append(samples, metricsdb.Sample{SubjectType: "device", SubjectID: d.ID, TenantID: d.OrganizationID, MetricName: "http_cert_expiry_days", Value: float64(*result.CertExpiryInDays), RecordedAt: now})
			}
			// Certificate details (issuer/subject/validity/fingerprint/chain)
			// are mostly-static metadata, not a time series -- persist the
			// latest snapshot on the device row rather than as metric
			// samples (see devices.CertInfo).
			if certInfo := NewCertInfo(result); certInfo != nil {
				if err := repo.UpdateHTTPCertInfo(ctx, d.ID, certInfo); err != nil {
					log.Printf("device metric sampler: update cert info for %s: %v", d.ID, err)
				}
			}
		}
	}
	if err := metrics.RecordBatch(ctx, samples); err != nil {
		log.Printf("device metric sampler: record batch: %v", err)
	}
}
