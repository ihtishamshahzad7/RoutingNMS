package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/accesspoints"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/alerts"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/alertsfeed"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/assistant"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/auth"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/customers"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/discovery"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/incidents"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/maintenance"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/metricsdb"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/mib"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/olt"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/ping"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/provisioning"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/sites"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmptrap"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/statuspage"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/syslog"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/tags"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/topology"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/traceroute"
	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var db *pgxpool.Pool
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			log.Fatalf("invalid DATABASE_URL: %v", err)
		}
		cfg.MaxConns = int32(envInt("DB_MAX_CONNS", 20))
		cfg.MinConns = int32(envInt("DB_MIN_CONNS", 2))
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		db, err = pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			log.Fatalf("database pool: %v", err)
		}
		if err := db.Ping(pingCtx); err != nil {
			db.Close()
			log.Fatalf("database ping: %v", err)
		}
		defer db.Close()
		log.Printf("PostgreSQL connection ready")
	} else {
		log.Printf("DATABASE_URL is not set; starting API without database")
	}

	var oltRuntime *olt.RuntimeManager
	var pingPoller *ping.Poller
	var topologyEngine *topology.Discovery
	var incidentEngine *incidents.Engine
	var incidentStream *incidents.Stream
	var alertEvaluator *alerts.Evaluator
	var authHandler auth.Handler
	if db != nil {
		profiles := olt.DefaultProfileRegistry()
		config := olt.ConfigService{DB: db, Profiles: profiles}
		oltRuntime = olt.NewRuntimeManager(config, olt.Repository{DB: db})
		oltRuntime.Metrics = olt.MetricSampler{Repo: metricsdb.Repository{DB: db}}
		if err := oltRuntime.Start(ctx); err != nil {
			log.Printf("OLT runtime initialization failed: %v", err)
		}
		log.Printf("OLT runtime manager started; active pollers=%d", oltRuntime.Running())
		authStore := auth.Store{DB: db}
		bootstrapCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := authStore.Bootstrap(bootstrapCtx); err != nil {
			log.Printf("auth bootstrap failed: %v", err)
		}
		cancel()
		authHandler = auth.Handler{Store: authStore, Secure: strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true")}
		go pruneSessionsPeriodically(ctx, authStore)

		// Syslog receiver: OLTs/routers/switches/CMTS can be pointed at this
		// NMS as a syslog target. Defaults to :1514 (no elevated privileges
		// needed); set SYSLOG_ADDR=":514" and grant the service
		// CAP_NET_BIND_SERVICE (see routingnms-api.service) to use the
		// standard port instead.
		syslogAddr := os.Getenv("SYSLOG_ADDR")
		if syslogAddr == "" {
			syslogAddr = ":1514"
		}
		go func() {
			if err := syslog.ListenAndServe(ctx, db, syslogAddr); err != nil {
				log.Printf("syslog receiver failed to start on %s: %v", syslogAddr, err)
			}
		}()
		go pruneSyslogPeriodically(ctx, db)

		// SNMP trap listener: OLTs/routers/switches/UPS controllers etc. can
		// send v1/v2c/v3 traps here; matched against trap_rules for
		// severity and stored for history. Defaults to :1162 (no elevated
		// privileges needed); set TRAP_ADDR=":162" and grant the service
		// CAP_NET_BIND_SERVICE (see routingnms-api.service) to use the
		// standard port instead.
		trapAddr := os.Getenv("TRAP_ADDR")
		if trapAddr == "" {
			trapAddr = ":1162"
		}
		trapFallbackAddr := os.Getenv("TRAP_FALLBACK_ADDR")
		if trapFallbackAddr == "" {
			trapFallbackAddr = ":1162"
		}
		trapListener := snmptrap.Listener{Repo: snmptrap.Repository{DB: db}}
		go snmptrap.ListenWithFallback(ctx, trapListener, trapAddr, trapFallbackAddr)
		go pruneTrapsPeriodically(ctx, db)

		// Per-device metric history: periodically probes every enabled
		// device (same health check as GET /devices/health) and records
		// up/latency samples, powering the charts on each device's page.
		// OLT/ONU optical metrics are recorded from the OLT poller above
		// instead (oltRuntime.Metrics), since that already has real data
		// every poll cycle.
		deviceMetricsInterval := time.Duration(envInt("DEVICE_METRICS_INTERVAL_SECONDS", 60)) * time.Second
		go devices.SamplePeriodically(ctx, devices.Repository{DB: db}, metricsdb.Repository{DB: db}, deviceMetricsInterval)
		go pruneMetricsPeriodically(ctx, db)

		// ICMP ping poller (the "pingmonitor" concept): probes every enabled
		// device that has icmp_enabled=true via the system `ping` binary
		// every PING_POLL_INTERVAL_SECONDS (default 30s), stores fine-grained
		// history in ping_results and records icmp_* metric samples for the
		// RTT sparklines on the device page. The existing TCP-only probing
		// above remains the unprivileged default for devices with ICMP
		// disabled.
		pingRepo := ping.Repository{DB: db}
		pingPoller = ping.New(pingRepo, metricsdb.Repository{DB: db})
		pingPollInterval := time.Duration(envInt("PING_POLL_INTERVAL_SECONDS", 30)) * time.Second
		go pingPoller.Run(ctx, pingPollInterval)
		go prunePingResultsPeriodically(ctx, db)

		// Sprint 1 — scheduled LLDP topology discovery. Walks the LLDP-MIB of
		// every SNMP-enabled device every TOPOLOGY_DISCOVER_INTERVAL_SECONDS
		// (default 15m = 900s), persists discovered links into topology_links,
		// and records a graph snapshot. The topology page reads these links via
		// GET /api/topology (now served from persistence instead of empty).
		topologyEngine = topology.NewDiscovery(topology.Repository{DB: db})
		topologyDiscInterval := time.Duration(envInt("TOPOLOGY_DISCOVER_INTERVAL_SECONDS", 900)) * time.Second
		topologyEngine.Interval = topologyDiscInterval
		go topologyEngine.Run(ctx)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{"ok", "routingnms-api", "0.1.0"})
	})
	mux.HandleFunc("GET /api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		status, code := "ready", http.StatusOK
		if db != nil {
			if err := db.Ping(r.Context()); err != nil {
				status, code = "not_ready", http.StatusServiceUnavailable
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})

	if db != nil {
		mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
		mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
		mux.Handle("GET /api/v1/auth/me", authHandler.OptionalMiddleware(http.HandlerFunc(authHandler.Me)))

		deviceHandler := devices.Handler{Repo: devices.Repository{DB: db}}
		discoveryHandler := devices.DiscoveryHandler{Repo: devices.Repository{DB: db}}
		healthHandler := devices.HealthHandler{Repo: devices.Repository{DB: db}}
		mux.Handle("POST /api/v1/devices", authHandler.Middleware(http.HandlerFunc(deviceHandler.Create)))
		mux.Handle("GET /api/v1/devices", authHandler.Middleware(http.HandlerFunc(deviceHandler.List)))
		mux.Handle("GET /api/v1/devices/health", authHandler.Middleware(http.HandlerFunc(healthHandler.ServeHTTP)))
		mux.Handle("POST /api/v1/devices/test", authHandler.Middleware(http.HandlerFunc(devices.TestHandler{}.ServeHTTP)))
		mux.Handle("PUT /api/v1/devices/", authHandler.Middleware(http.HandlerFunc(deviceHandler.UpdateSNMP)))
		mux.Handle("PUT /api/v1/devices/{id}/http-check", authHandler.Middleware(http.HandlerFunc(deviceHandler.UpdateHTTPCheck)))
		mux.Handle("POST /api/v1/devices/", authHandler.Middleware(http.HandlerFunc(discoveryHandler.Discover)))
		mux.Handle("GET /api/v1/devices/", authHandler.Middleware(http.HandlerFunc(discoveryHandler.Interfaces)))

		mux.Handle("GET /api/v1/olt/runtime", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			states := []olt.RuntimeState{}
			running := 0
			if oltRuntime != nil {
				states, running = oltRuntime.States(), oltRuntime.Running()
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"running": running, "olts": states})
		})))
		mux.Handle("GET /api/v1/olts/", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimRight(r.URL.Path, "/")
			if strings.HasSuffix(path, "/alerts") {
				(olt.AlertAPI{DB: db}).ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
		})))

		// Add/list OLTs (the "Add OLT" flow: the config_service.Create +
		// RuntimeManager.StartOne wiring existed unused in the codebase
		// before this — this is what actually exposes it over HTTP).
		// Registered as /api/v1/olts (note: distinct from /api/v1/olts/
		// above, which only serves the nested /alerts sub-route).
		oltCreate := olt.CreateHandler{Config: olt.ConfigService{DB: db, Profiles: olt.DefaultProfileRegistry()}, Runtime: oltRuntime}
		mux.Handle("GET /api/v1/olts", authHandler.Middleware(oltCreate))
		mux.Handle("POST /api/v1/olts", authHandler.Middleware(oltCreate))

		// Per-OLT PON/ONU hierarchy drill-down, as called by
		// frontend/app/olts/[id]/page.tsx.
		oltHierarchy := olt.API{Provider: olt.DBProvider{DB: db}}
		mux.Handle("GET /api/olts/", authHandler.Middleware(http.StripPrefix("/api", oltHierarchy)))

		// Incident lifecycle (open/acknowledge/resolve) and live SSE
		// stream, as called by frontend/app/incidents/page.tsx and the
		// (currently unmounted-in-the-UI) live-notification components.
		// The engine + stream are hoisted to function scope so Sprint 2's
		// alert evaluator can feed incidents into them.
		incidentEngine = incidents.NewEngine()
		incidentStream = incidents.NewStream()
		incidentAPI := http.StripPrefix("/api", incidents.API{Engine: incidentEngine})
		mux.Handle("GET /api/incidents", authHandler.Middleware(incidentAPI))
		mux.Handle("GET /api/incidents/stream", authHandler.Middleware(incidentStream))
		mux.Handle("GET /api/incidents/", authHandler.Middleware(incidentAPI))
		mux.Handle("POST /api/incidents/", authHandler.Middleware(incidentAPI))

		// Topology graph, as called by frontend/app/topology/page.tsx.
		// Built from real registered inventory (devices + OLTs) with active
		// links now sourced from the topology_links table, which the
		// scheduled LLDP discovery loop (started above) keeps up to date.
		topologyRepo := topology.Repository{DB: db}
		mux.Handle("GET /api/topology", authHandler.Middleware(topology.GraphHandler{Repo: topologyRepo}))

		// Sprint 1 topology administration: manual rediscovery, last-cycle
		// status, and the 48h snapshot history for time-travel.
		if topologyEngine != nil {
			mux.Handle("POST /api/v1/topology/discover", authHandler.Middleware(topology.DiscoverHandler{Engine: topologyEngine}))
			mux.Handle("GET /api/v1/topology/status", authHandler.Middleware(topology.StatusHandler{Engine: topologyEngine}))
		}
		mux.Handle("GET /api/v1/topology/snapshots", authHandler.Middleware(topology.SnapshotHandler{Repo: topologyRepo}))

		// Sprint 2 — generic alert rules + AI incident hub. The previously
		// dormant internal/alerts engine is now wired to persisted rules
		// (alert_rules, migration 0016), real metric history, notification
		// channels (0017), and the incident system above: fired alerts open
		// incidents in incidentEngine + publish to incidentStream's SSE, get
		// persisted with RCA into ai_incidents (0015) for the Incident Hub.
		alertRepo := alerts.Repository{DB: db}
		alertEvaluator = alerts.NewEvaluator(alertRepo, incidentEngine, incidentStream)
		alertEvalInterval := time.Duration(envInt("ALERT_EVAL_INTERVAL_SECONDS", 60)) * time.Second
		alertEvaluator.Interval = alertEvalInterval
		go alertEvaluator.Run(ctx)
		alertsAPI := alerts.API{Repo: alertRepo, Evaluator: alertEvaluator}
		mux.Handle("GET /api/v1/alerts/", authHandler.Middleware(http.StripPrefix("/api/v1", alertsAPI)))
		mux.Handle("POST /api/v1/alerts/", authHandler.Middleware(http.StripPrefix("/api/v1", alertsAPI)))
		mux.Handle("PUT /api/v1/alerts/", authHandler.Middleware(http.StripPrefix("/api/v1", alertsAPI)))
		mux.Handle("DELETE /api/v1/alerts/", authHandler.Middleware(http.StripPrefix("/api/v1", alertsAPI)))

		// Recent syslog messages, as called by frontend/app/syslog/page.tsx.
		// Registered under /api/v1 (not bare /api) to match apiFetch's
		// API_BASE, which the syslog page uses -- the bare /api/syslog path
		// this used to be registered under never matched what the frontend
		// actually requested (Next.js's own 404 page was silently served by
		// the `/` nginx fallback instead of the API's real 404, and
		// SyslogPage's JSON parse then failed, always showing "No syslog
		// messages received yet." even with real data flowing in).
		mux.Handle("GET /api/v1/syslog", authHandler.Middleware(syslog.API{DB: db}))

		// SNMP trap history + alert rule engine, as called by
		// frontend/app/traps/page.tsx.
		trapRepo := snmptrap.Repository{DB: db}
		mux.Handle("GET /api/v1/traps/rules", authHandler.Middleware(snmptrap.RulesAPI{Repo: trapRepo}))
		mux.Handle("POST /api/v1/traps/rules", authHandler.Middleware(snmptrap.RulesAPI{Repo: trapRepo}))
		mux.Handle("DELETE /api/v1/traps/rules/{id}", authHandler.Middleware(snmptrap.RuleAPI{Repo: trapRepo}))
		mux.Handle("GET /api/v1/traps", authHandler.Middleware(snmptrap.TrapsAPI{Repo: trapRepo}))

		// MIB manager: upload vendor .mib/.my files, search by name/OID, and
		// a live OID tester against a real device, as called by
		// frontend/app/mibs/page.tsx.
		mibRepo := mib.Repository{DB: db}
		devicesRepo := devices.Repository{DB: db}
		mux.Handle("GET /api/v1/mibs", authHandler.Middleware(mib.API{Repo: mibRepo}))
		mux.Handle("POST /api/v1/mibs", authHandler.Middleware(mib.API{Repo: mibRepo}))
		mux.Handle("DELETE /api/v1/mibs/{id}", authHandler.Middleware(mib.MIBAPI{Repo: mibRepo}))
		mux.Handle("GET /api/v1/mibs/search", authHandler.Middleware(mib.SearchAPI{Repo: mibRepo}))
		mux.Handle("POST /api/v1/mibs/test", authHandler.Middleware(mib.TestAPI{Repo: mibRepo, Devices: devicesRepo, Collector: snmp.Collector{}}))

		// Per-device/OLT/ONU metric history, as called by the charts on
		// frontend/app/devices/[id]/page.tsx and frontend/app/olts/[id]/page.tsx.
		mux.Handle("GET /api/v1/metrics", authHandler.Middleware(metricsdb.API{Repo: metricsdb.Repository{DB: db}}))

		// ICMP ping live/history/probe, powering the "Ping" tab on the device
		// detail page (frontend/app/devices/[id]/page.tsx). Dispatched on the
		// URL suffix like the OLT nested /alerts route above.
		pingAPI := ping.API{Repo: ping.Repository{DB: db}, Devices: devicesRepo, Poller: pingPoller}
		mux.Handle("GET /api/v1/ping/", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/live"):
				pingAPI.Live(w, r)
			case strings.HasSuffix(r.URL.Path, "/history"):
				pingAPI.History(w, r)
			default:
				http.NotFound(w, r)
			}
		})))
		mux.Handle("POST /api/v1/ping/", authHandler.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/probe") {
				pingAPI.Probe(w, r)
				return
			}
			http.NotFound(w, r)
		})))

		// On-demand traceroute -- an "advanced" pinging capability Kuma's
		// ping monitor never had: hop-by-hop path trace to a device, for
		// diagnosing where a path breaks rather than just that it did.
		mux.Handle("POST /api/v1/devices/{id}/traceroute", authHandler.Middleware(traceroute.API{Devices: devicesRepo}))

		// Subnet auto-discovery: scan a CIDR over SNMP, classify what
		// responds, and one-click import selected hosts as devices, as
		// called by frontend/app/devices/page.tsx's "Discover subnet" flow.
		discoveryManager := discovery.NewManager()
		mux.Handle("POST /api/v1/discovery/scan", authHandler.Middleware(discovery.ScanAPI{Manager: discoveryManager}))
		mux.Handle("GET /api/v1/discovery/scan/{id}", authHandler.Middleware(discovery.JobAPI{Manager: discoveryManager}))
		mux.Handle("POST /api/v1/discovery/import", authHandler.Middleware(discovery.ImportAPI{Manager: discoveryManager, Devices: devicesRepo}))

		// Unified active-alerts feed (open OLT alerts + unreachable devices
		// + recent critical/warning SNMP traps), polled by the browser
		// voice-alert feature so it doesn't have to stitch three APIs
		// together itself.
		mux.Handle("GET /api/v1/alerts/active", authHandler.Middleware(alertsfeed.API{Repo: alertsfeed.Repository{DB: db, Maintenance: maintenance.Checker{DB: db}}}))

		// RouterOS auto-provisioning: admin pre-registers a router device
		// (with its serial number) and assigns a script template; the
		// router itself then pulls its config via `/tool fetch` using a
		// shared token (RouterOS has no session cookie, so it can't use
		// authHandler.Middleware like everything else here). There is
		// deliberately no auto-registration of unknown devices -- only a
		// device already known by serial number can be provisioned.
		provisioningRepo := provisioning.Repository{DB: db}
		provisionSalt := os.Getenv("PROVISION_SALT")
		if provisionSalt == "" {
			provisionSalt = "routingnms-dev-salt"
			log.Printf("PROVISION_SALT is not set; using an insecure default -- set it in production")
		}
		provisionToken := os.Getenv("PROVISION_TOKEN")
		provisionBaseURL := strings.TrimSuffix(os.Getenv("PUBLIC_API_BASE_URL"), "/")
		mux.Handle("GET /api/v1/provisioning/templates", authHandler.Middleware(provisioning.TemplatesAPI{Repo: provisioningRepo}))
		mux.Handle("POST /api/v1/provisioning/templates", authHandler.Middleware(provisioning.TemplatesAPI{Repo: provisioningRepo}))
		mux.Handle("GET /api/v1/provisioning/templates/{id}", authHandler.Middleware(provisioning.TemplatesAPI{Repo: provisioningRepo}))
		mux.Handle("PUT /api/v1/provisioning/templates/{id}", authHandler.Middleware(provisioning.TemplatesAPI{Repo: provisioningRepo}))
		mux.Handle("DELETE /api/v1/provisioning/templates/{id}", authHandler.Middleware(provisioning.TemplatesAPI{Repo: provisioningRepo}))
		mux.Handle("PUT /api/v1/devices/{id}/provisioning", authHandler.Middleware(provisioning.AssignAPI{Templates: provisioningRepo, Devices: devicesRepo}))
		mux.Handle("GET /api/v1/devices/{id}/provisioning/preview", authHandler.Middleware(provisioning.PreviewAPI{Templates: provisioningRepo, Devices: devicesRepo, Salt: provisionSalt, BaseURL: provisionBaseURL, Token: provisionToken}))
		mux.Handle("GET /api/v1/provision/routeros/{serial}", provisioning.FetchAPI{Templates: provisioningRepo, Devices: devicesRepo, Salt: provisionSalt, Token: provisionToken})

		// Public status pages, ported from Uptime Kuma: a branded,
		// unauthenticated page listing chosen devices/OLTs and their
		// current status. Admin CRUD is session-authed; the public view
		// (GET /api/v1/public/status/{slug}) is registered separately,
		// outside the auth-required block below.
		statusPageRepo := statuspage.Repository{DB: db}
		mux.Handle("GET /api/v1/status-pages", authHandler.Middleware(statuspage.AdminAPI{Repo: statusPageRepo}))
		mux.Handle("POST /api/v1/status-pages", authHandler.Middleware(statuspage.AdminAPI{Repo: statusPageRepo}))
		mux.Handle("GET /api/v1/status-pages/{id}", authHandler.Middleware(statuspage.AdminAPI{Repo: statusPageRepo}))
		mux.Handle("PUT /api/v1/status-pages/{id}", authHandler.Middleware(statuspage.AdminAPI{Repo: statusPageRepo}))
		mux.Handle("DELETE /api/v1/status-pages/{id}", authHandler.Middleware(statuspage.AdminAPI{Repo: statusPageRepo}))
		mux.Handle("PUT /api/v1/status-pages/{id}/items", authHandler.Middleware(statuspage.ItemsAPI{Repo: statusPageRepo}))
		mux.Handle("GET /api/v1/public/status/{slug}", statuspage.PublicAPI{Repo: statusPageRepo, Resolver: statuspage.StatusResolver{DB: db}})

		maintenanceRepo := maintenance.Repository{DB: db}
		mux.Handle("GET /api/v1/maintenance-windows", authHandler.Middleware(maintenance.AdminAPI{Repo: maintenanceRepo}))
		mux.Handle("POST /api/v1/maintenance-windows", authHandler.Middleware(maintenance.AdminAPI{Repo: maintenanceRepo}))
		mux.Handle("GET /api/v1/maintenance-windows/{id}", authHandler.Middleware(maintenance.AdminAPI{Repo: maintenanceRepo}))
		mux.Handle("PUT /api/v1/maintenance-windows/{id}", authHandler.Middleware(maintenance.AdminAPI{Repo: maintenanceRepo}))
		mux.Handle("DELETE /api/v1/maintenance-windows/{id}", authHandler.Middleware(maintenance.AdminAPI{Repo: maintenanceRepo}))
		mux.Handle("PUT /api/v1/maintenance-windows/{id}/items", authHandler.Middleware(maintenance.ItemsAPI{Repo: maintenanceRepo}))

		tagsRepo := tags.Repository{DB: db}
		mux.Handle("GET /api/v1/tags", authHandler.Middleware(tags.AdminAPI{Repo: tagsRepo}))
		mux.Handle("POST /api/v1/tags", authHandler.Middleware(tags.AdminAPI{Repo: tagsRepo}))
		mux.Handle("PUT /api/v1/tags/{id}", authHandler.Middleware(tags.AdminAPI{Repo: tagsRepo}))
		mux.Handle("DELETE /api/v1/tags/{id}", authHandler.Middleware(tags.AdminAPI{Repo: tagsRepo}))
		mux.Handle("GET /api/v1/tags/assignments", authHandler.Middleware(tags.AssignmentsAPI{Repo: tagsRepo}))
		mux.Handle("GET /api/v1/tag-assignments/{subjectType}/{subjectId}", authHandler.Middleware(tags.AssignmentsAPI{Repo: tagsRepo}))
		mux.Handle("PUT /api/v1/tag-assignments/{subjectType}/{subjectId}", authHandler.Middleware(tags.AssignmentsAPI{Repo: tagsRepo}))

		// Sprint 3 — ISP features: physical sites, wireless access points,
		// and subscriber customer connections (migration 0018). Session-authed
		// CRUD following the provisioning/templates idiom ({id} path vars).
		sitesRepo := sites.Repository{DB: db}
		mux.Handle("GET /api/v1/sites", authHandler.Middleware(sites.API{Repo: sitesRepo}))
		mux.Handle("POST /api/v1/sites", authHandler.Middleware(sites.API{Repo: sitesRepo}))
		mux.Handle("GET /api/v1/sites/{id}", authHandler.Middleware(sites.API{Repo: sitesRepo}))
		mux.Handle("PUT /api/v1/sites/{id}", authHandler.Middleware(sites.API{Repo: sitesRepo}))
		mux.Handle("DELETE /api/v1/sites/{id}", authHandler.Middleware(sites.API{Repo: sitesRepo}))

		accessPointsRepo := accesspoints.Repository{DB: db}
		mux.Handle("GET /api/v1/access-points", authHandler.Middleware(accesspoints.API{Repo: accessPointsRepo}))
		mux.Handle("POST /api/v1/access-points", authHandler.Middleware(accesspoints.API{Repo: accessPointsRepo}))
		mux.Handle("GET /api/v1/access-points/{id}", authHandler.Middleware(accesspoints.API{Repo: accessPointsRepo}))
		mux.Handle("PUT /api/v1/access-points/{id}", authHandler.Middleware(accesspoints.API{Repo: accessPointsRepo}))
		mux.Handle("DELETE /api/v1/access-points/{id}", authHandler.Middleware(accesspoints.API{Repo: accessPointsRepo}))

		customersRepo := customers.Repository{DB: db}
		mux.Handle("GET /api/v1/customers", authHandler.Middleware(customers.API{Repo: customersRepo}))
		mux.Handle("POST /api/v1/customers", authHandler.Middleware(customers.API{Repo: customersRepo}))
		mux.Handle("GET /api/v1/customers/{id}", authHandler.Middleware(customers.API{Repo: customersRepo}))
		mux.Handle("PUT /api/v1/customers/{id}", authHandler.Middleware(customers.API{Repo: customersRepo}))
		mux.Handle("DELETE /api/v1/customers/{id}", authHandler.Middleware(customers.API{Repo: customersRepo}))

		// Sprint 4 — NOC AI assistant (Screen 5 chat widget): deterministic,
		// backend-grounded answers built from the live active alert feed +
		// durable AI incidents. No external model at runtime.
		assistantRepo := assistant.Repository{DB: db}
		mux.Handle("POST /api/v1/ai/assistant", authHandler.Middleware(assistant.API{Repo: assistantRepo}))

	} else {
		unavailable := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "database not configured"})
		}
		mux.HandleFunc("POST /api/v1/auth/login", unavailable)
		mux.HandleFunc("GET /api/v1/auth/me", unavailable)
		mux.HandleFunc("GET /api/v1/devices", unavailable)
		mux.HandleFunc("POST /api/v1/devices", unavailable)
		mux.HandleFunc("GET /api/v1/devices/health", unavailable)
		mux.HandleFunc("POST /api/v1/devices/test", unavailable)
		mux.HandleFunc("PUT /api/v1/devices/", unavailable)
		mux.HandleFunc("PUT /api/v1/devices/{id}/http-check", unavailable)
		mux.HandleFunc("POST /api/v1/devices/", unavailable)
		mux.HandleFunc("GET /api/v1/devices/", unavailable)
		mux.HandleFunc("GET /api/v1/olt/runtime", unavailable)
		mux.HandleFunc("GET /api/v1/olts/", unavailable)
		mux.HandleFunc("GET /api/v1/olts", unavailable)
		mux.HandleFunc("POST /api/v1/olts", unavailable)
		mux.HandleFunc("GET /api/olts/", unavailable)
		mux.HandleFunc("GET /api/incidents", unavailable)
		mux.HandleFunc("GET /api/incidents/stream", unavailable)
		mux.HandleFunc("GET /api/incidents/", unavailable)
		mux.HandleFunc("POST /api/incidents/", unavailable)
		mux.HandleFunc("GET /api/topology", unavailable)
		mux.HandleFunc("GET /api/v1/syslog", unavailable)
		mux.HandleFunc("GET /api/v1/traps/rules", unavailable)
		mux.HandleFunc("POST /api/v1/traps/rules", unavailable)
		mux.HandleFunc("DELETE /api/v1/traps/rules/{id}", unavailable)
		mux.HandleFunc("GET /api/v1/traps", unavailable)
		mux.HandleFunc("GET /api/v1/mibs", unavailable)
		mux.HandleFunc("POST /api/v1/mibs", unavailable)
		mux.HandleFunc("DELETE /api/v1/mibs/{id}", unavailable)
		mux.HandleFunc("GET /api/v1/mibs/search", unavailable)
		mux.HandleFunc("POST /api/v1/mibs/test", unavailable)
		mux.HandleFunc("GET /api/v1/metrics", unavailable)
		mux.HandleFunc("GET /api/v1/ping/", unavailable)
		mux.HandleFunc("POST /api/v1/ping/", unavailable)
		mux.HandleFunc("POST /api/v1/discovery/scan", unavailable)
		mux.HandleFunc("GET /api/v1/discovery/scan/{id}", unavailable)
		mux.HandleFunc("POST /api/v1/discovery/import", unavailable)
		mux.HandleFunc("GET /api/v1/alerts/active", unavailable)
		mux.HandleFunc("GET /api/v1/provisioning/templates", unavailable)
		mux.HandleFunc("POST /api/v1/provisioning/templates", unavailable)
		mux.HandleFunc("GET /api/v1/provisioning/templates/{id}", unavailable)
		mux.HandleFunc("PUT /api/v1/provisioning/templates/{id}", unavailable)
		mux.HandleFunc("DELETE /api/v1/provisioning/templates/{id}", unavailable)
		mux.HandleFunc("PUT /api/v1/devices/{id}/provisioning", unavailable)
		mux.HandleFunc("GET /api/v1/devices/{id}/provisioning/preview", unavailable)
		mux.HandleFunc("POST /api/v1/devices/{id}/traceroute", unavailable)
		mux.HandleFunc("GET /api/v1/provision/routeros/{serial}", unavailable)
		mux.HandleFunc("GET /api/v1/status-pages", unavailable)
		mux.HandleFunc("POST /api/v1/status-pages", unavailable)
		mux.HandleFunc("GET /api/v1/status-pages/{id}", unavailable)
		mux.HandleFunc("PUT /api/v1/status-pages/{id}", unavailable)
		mux.HandleFunc("DELETE /api/v1/status-pages/{id}", unavailable)
		mux.HandleFunc("PUT /api/v1/status-pages/{id}/items", unavailable)
		mux.HandleFunc("GET /api/v1/public/status/{slug}", unavailable)
		mux.HandleFunc("GET /api/v1/maintenance-windows", unavailable)
		mux.HandleFunc("POST /api/v1/maintenance-windows", unavailable)
		mux.HandleFunc("GET /api/v1/maintenance-windows/{id}", unavailable)
		mux.HandleFunc("PUT /api/v1/maintenance-windows/{id}", unavailable)
		mux.HandleFunc("DELETE /api/v1/maintenance-windows/{id}", unavailable)
		mux.HandleFunc("PUT /api/v1/maintenance-windows/{id}/items", unavailable)
		mux.HandleFunc("GET /api/v1/tags", unavailable)
		mux.HandleFunc("POST /api/v1/tags", unavailable)
		mux.HandleFunc("PUT /api/v1/tags/{id}", unavailable)
		mux.HandleFunc("DELETE /api/v1/tags/{id}", unavailable)
		mux.HandleFunc("GET /api/v1/tags/assignments", unavailable)
		mux.HandleFunc("GET /api/v1/tag-assignments/{subjectType}/{subjectId}", unavailable)
		mux.HandleFunc("PUT /api/v1/tag-assignments/{subjectType}/{subjectId}", unavailable)
		mux.HandleFunc("POST /api/v1/ai/assistant", unavailable)
	}

	srv := &http.Server{Addr: ":" + port, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("RoutingNMS API listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	if oltRuntime != nil {
		oltRuntime.Stop()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
func envInt(name string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(name))
	if err != nil || v < 1 {
		return fallback
	}
	return v
}
func pruneSessionsPeriodically(ctx context.Context, store auth.Store) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := store.PruneExpired(ctx); err != nil {
				log.Printf("prune expired sessions: %v", err)
			}
		}
	}
}
func pruneSyslogPeriodically(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	const retention = 14 * 24 * time.Hour
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := syslog.PruneOlderThan(ctx, db, retention); err != nil {
				log.Printf("prune syslog messages: %v", err)
			} else if n > 0 {
				log.Printf("pruned %d syslog messages older than %s", n, retention)
			}
		}
	}
}
func pruneTrapsPeriodically(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	const retention = 30 * 24 * time.Hour
	repo := snmptrap.Repository{DB: db}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := repo.PruneOlderThan(ctx, retention); err != nil {
				log.Printf("prune snmp traps: %v", err)
			} else if n > 0 {
				log.Printf("pruned %d snmp traps older than %s", n, retention)
			}
		}
	}
}
func pruneMetricsPeriodically(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	const retention = 30 * 24 * time.Hour
	repo := metricsdb.Repository{DB: db}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := repo.PruneOlderThan(ctx, retention); err != nil {
				log.Printf("prune metric samples: %v", err)
			} else if n > 0 {
				log.Printf("pruned %d metric samples older than %s", n, retention)
			}
		}
	}
}
func prunePingResultsPeriodically(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	const retention = 7 * 24 * time.Hour
	repo := ping.Repository{DB: db}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := repo.PruneOlderThan(ctx, retention); err != nil {
				log.Printf("prune ping results: %v", err)
			} else if n > 0 {
				log.Printf("pruned %d ping results older than %s", n, retention)
			}
		}
	}
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
