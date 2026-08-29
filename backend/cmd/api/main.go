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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/olt"
)

type healthResponse struct { Status string `json:"status"`; Service string `json:"service"`; Version string `json:"version"` }

func main() {
	port := os.Getenv("APP_PORT"); if port == "" { port = "8080" }
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM); defer stop()
	var db *pgxpool.Pool
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		cfg, err := pgxpool.ParseConfig(dsn); if err != nil { log.Fatalf("invalid DATABASE_URL: %v", err) }
		cfg.MaxConns = int32(envInt("DB_MAX_CONNS", 20)); cfg.MinConns = int32(envInt("DB_MIN_CONNS", 2))
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second); defer cancel()
		db, err = pgxpool.NewWithConfig(ctx, cfg); if err != nil { log.Fatalf("database pool: %v", err) }
		if err := db.Ping(pingCtx); err != nil { db.Close(); log.Fatalf("database ping: %v", err) }
		defer db.Close(); log.Printf("PostgreSQL connection ready")
	} else { log.Printf("DATABASE_URL is not set; starting API without database") }

	var oltRuntime *olt.RuntimeManager
	if db != nil {
		profiles := olt.DefaultProfileRegistry()
		config := olt.ConfigService{DB: db, Profiles: profiles}
		oltRuntime = olt.NewRuntimeManager(config, olt.Repository{DB: db})
		if err := oltRuntime.Start(ctx); err != nil { log.Printf("OLT runtime initialization failed: %v", err) }
		log.Printf("OLT runtime manager started; active pollers=%d", oltRuntime.Running())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(healthResponse{"ok","routingnms-api","0.1.0"}) })
	mux.HandleFunc("GET /api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		status := "ready"; code := http.StatusOK
		if db != nil { if err := db.Ping(r.Context()); err != nil { status="not_ready"; code=http.StatusServiceUnavailable } }
		w.Header().Set("Content-Type","application/json"); w.WriteHeader(code); _=json.NewEncoder(w).Encode(map[string]string{"status":status})
	})
	mux.HandleFunc("GET /api/v1/olt/runtime", func(w http.ResponseWriter, r *http.Request) {
		states := []olt.RuntimeState{}; running := 0
		if oltRuntime != nil { states = oltRuntime.States(); running = oltRuntime.Running() }
		w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(map[string]any{"running":running,"olts":states})
	})
	mux.Handle("GET /api/v1/olts/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimRight(r.URL.Path, "/")
		if strings.HasSuffix(path, "/alerts") { (olt.AlertAPI{DB: db}).ServeHTTP(w,r); return }
		http.NotFound(w,r)
	}))

	srv := &http.Server{Addr:":"+port, Handler:securityHeaders(mux), ReadHeaderTimeout:5*time.Second, ReadTimeout:15*time.Second, WriteTimeout:15*time.Second, IdleTimeout:60*time.Second}
	go func(){ log.Printf("RoutingNMS API listening on :%s",port); if err:=srv.ListenAndServe(); err!=nil && err!=http.ErrServerClosed { log.Fatal(err) } }()
	<-ctx.Done()
	if oltRuntime != nil { oltRuntime.Stop() }
	shutdownCtx,cancel:=context.WithTimeout(context.Background(),10*time.Second); defer cancel(); _=srv.Shutdown(shutdownCtx)
}

func envInt(name string, fallback int) int { v,err:=strconv.Atoi(os.Getenv(name)); if err!=nil || v<1{return fallback}; return v }
func securityHeaders(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ w.Header().Set("X-Content-Type-Options","nosniff"); w.Header().Set("X-Frame-Options","DENY"); w.Header().Set("Referrer-Policy","strict-origin-when-cross-origin"); next.ServeHTTP(w,r) }) }
