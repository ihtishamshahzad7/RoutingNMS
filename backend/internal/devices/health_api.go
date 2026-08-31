package devices

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/monitor"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type DeviceHealth struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	DeviceType string    `json:"deviceType"`
	Vendor     string    `json:"vendor,omitempty"`
	Method     string    `json:"method"`
	Reachable  bool      `json:"reachable"`
	LatencyMS  float64   `json:"latencyMs"`
	CheckedAt  time.Time `json:"checkedAt"`
	Error      string    `json:"error,omitempty"`
}

type HealthHandler struct{ Repo Repository }

func (h HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	org := strings.TrimSpace(r.URL.Query().Get("organizationId"))
	if org == "" {
		http.Error(w, "organizationId is required", http.StatusBadRequest)
		return
	}
	devices, err := h.Repo.List(r.Context(), org)
	if err != nil {
		http.Error(w, "failed to load devices: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]DeviceHealth, len(devices))
	var wg sync.WaitGroup
	for i, device := range devices {
		wg.Add(1)
		go func(i int, d Record) {
			defer wg.Done()
			out[i] = ProbeDevice(r.Context(), h.Repo, d)
		}(i, device)
	}
	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func ProbeDevice(ctx context.Context, repo Repository, d Record) DeviceHealth {
	result := DeviceHealth{ID: d.ID, Name: d.Name, Address: d.Address, DeviceType: d.DeviceType, Vendor: d.Vendor, CheckedAt: time.Now().UTC()}
	started := time.Now()
	timeout := 3 * time.Second
	if d.SNMPEnabled {
		result.Method = "SNMP"
		input, err := repo.DiscoveryTarget(ctx, d.ID)
		if err == nil {
			client, connectErr := (snmp.Collector{}).Connect(ctx, snmp.Target{ID: input.Name, Address: input.Address, Port: input.SNMPPort, Credentials: input.SNMP, Timeout: input.Timeout, Retries: 0})
			if connectErr == nil {
				result.Reachable = true
				_ = client.Conn.Close()
			} else {
				result.Error = connectErr.Error()
			}
		} else {
			result.Error = err.Error()
		}
	} else {
		result.Method = "TCP"
		probe := monitor.Ping(ctx, d.Address, timeout)
		result.Reachable = probe.Reachable
		result.Error = probe.Error
	}
	result.LatencyMS = float64(time.Since(started).Microseconds()) / 1000
	return result
}
