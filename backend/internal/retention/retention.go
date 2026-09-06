// Package retention implements RoutingNMS's equivalent of Uptime Kuma's
// clear-old-data background job (server/jobs/clear-old-data.js): a
// recurring pass that deletes old time-series history so the database
// doesn't grow forever.
//
// Kuma deletes `heartbeat` rows older than a single, instance-wide
// `keepDataPeriodDays` setting (default 180 days, disabled entirely when set
// below 1). RoutingNMS is multi-tenant, so this job walks every tenant and
// applies that tenant's own retention period to its metric_samples rows
// (metric_samples is this project's equivalent of `heartbeat`: it stores
// ICMP/SNMP/HTTP/DNS/push/SSH/Telnet/topology-link samples -- see
// internal/metricsdb). Metric samples that predate per-sample tenant
// attribution, or whose owning device/OLT can't be resolved to a tenant
// (tenant_id=""), are swept separately using the same default period, so
// they don't grow unbounded either.
package retention

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/tenants"
)

// Run performs one cleanup pass: for every known tenant, delete that
// tenant's metric_samples rows older than its configured retention period
// (default tenants.DefaultRetentionDays days if unset), skipping tenants
// whose period is set below 1 day (retention disabled, matching Kuma's
// "period < 1 -> keep forever" behavior). Also sweeps unattributed
// (tenant_id="") rows using the default period. Errors are logged, not
// returned, so one failed tenant or one failed pass never crashes the
// caller's background loop.
func Run(ctx context.Context, db *pgxpool.Pool) {
	tenantsRepo := tenants.Repository{DB: db}
	metrics := metricsdb.Repository{DB: db}

	ids, err := tenantsRepo.AllTenantIDs(ctx)
	if err != nil {
		log.Printf("retention: list tenants: %v", err)
		return
	}

	// Unattributed rows (tenant_id="") -- samples recorded before per-sample
	// tenant attribution existed, or whose subject type (OLT/PON/ONU) isn't
	// resolved to a tenant yet -- are cleaned up on the default period so
	// they don't grow forever, but aren't individually configurable since
	// they don't belong to any one tenant.
	ids = append(ids, "")

	for _, id := range ids {
		days := tenants.DefaultRetentionDays
		if id != "" {
			d, err := tenantsRepo.RetentionDays(ctx, id)
			if err != nil {
				log.Printf("retention: read retention period for tenant %q: %v", id, err)
				continue
			}
			days = d
		}
		if days < 1 {
			// Retention disabled for this tenant -- keep data forever,
			// matching Kuma's keepDataPeriodDays < 1 semantics.
			continue
		}
		cutoff := time.Now().UTC().AddDate(0, 0, -days)
		n, err := metrics.DeleteOlderThan(ctx, id, cutoff)
		if err != nil {
			log.Printf("retention: delete old metric_samples for tenant %q: %v", id, err)
			continue
		}
		if n > 0 {
			log.Printf("retention: deleted %d metric_samples row(s) older than %d day(s) for tenant %q", n, days, id)
		}
	}
}

// RunPeriodically runs Run once immediately and then every interval until
// ctx is done, mirroring the "run once at startup, then on a ticker" shape
// already used by this codebase's other background jobs (e.g.
// devices.SamplePeriodically, topolinks.Poller.Run).
func RunPeriodically(ctx context.Context, db *pgxpool.Pool, interval time.Duration) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	Run(ctx, db)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			Run(ctx, db)
		}
	}
}
