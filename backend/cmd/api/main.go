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

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/auth"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/incidents"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/olt"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/syslog"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/topology"
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
	var authHandler auth.Handler
	if db != nil {
		profiles := olt.DefaultProfileRegistry()
		config := olt.ConfigService{DB: db, Profiles: profiles}
		oltRuntime = olt.NewRuntimeManager(config, olt.Repository{DB: db})
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
		incidentEngine := incidents.NewEngine()
		incidentStream := incidents.NewStream()
		incidentAPI := http.StripPrefix("/api", incidents.API{Engine: incidentEngine})
		mux.Handle("GET /api/incidents", authHandler.Middleware(incidentAPI))
		mux.Handle("GET /api/incidents/stream", authHandler.Middleware(incidentStream))
		mux.Handle("GET /api/incidents/", authHandler.Middleware(incidentAPI))
		mux.Handle("POST /api/incidents/", authHandler.Middleware(incidentAPI))

		// Topology graph, as called by frontend/app/topology/page.tsx.
		// Built from real registered inventory (devices + OLTs); links are
		// empty until a scheduled LLDP discovery loop is wired up, rather
		// than inventing connections that were never actually discovered.
		mux.Handle("GET /api/topology", authHandler.Middleware(topology.API{Graph: topology.LiveGraph(db)}))

		// Recent syslog messages, as called by frontend/app/syslog/page.tsx.
		mux.Handle("GET /api/syslog", authHandler.Middleware(syslog.API{DB: db}))
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
		mux.HandleFunc("GET /api/syslog", unavailable)
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
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
