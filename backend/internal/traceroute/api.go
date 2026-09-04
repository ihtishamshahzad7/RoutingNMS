package traceroute

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
)

// API backs POST /api/v1/devices/{id}/traceroute -- runs an on-demand trace
// to that device's address and returns the hop list.
type API struct {
	Devices devices.Repository
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "device id is required", http.StatusBadRequest)
		return
	}
	dev, err := a.Devices.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	res := Run(ctx, dev.Address, 20)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
