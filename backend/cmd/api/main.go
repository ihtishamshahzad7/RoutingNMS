package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type healthResponse struct {
	Status string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status: "ok",
			Service: "routingnms-api",
			Version: "0.1.0",
		})
	})

	mux.HandleFunc("GET /api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	log.Printf("RoutingNMS API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, securityHeaders(mux)); err != nil {
		log.Fatal(err)
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
