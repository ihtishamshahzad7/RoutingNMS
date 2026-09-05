package dnscheck

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
)

// API exposes DNS-check live status and on-demand checks for the frontend's
// device detail page, mirroring ping.API's shape.
type API struct {
	Devices devices.Repository
	Poller  *Poller
}

// Live serves GET /api/v1/dns/{id}/live -- the most recent check result.
func (a API) Live(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r.URL.Path, "live")
	if !ok {
		http.NotFound(w, r)
		return
	}
	var live Result
	if a.Poller != nil {
		live, _ = a.Poller.Live(id)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"live": live})
}

// Check serves POST /api/v1/dns/{id}/check -- forces an immediate DNS
// resolution check and returns the result (used by a "Check now" button).
func (a API) Check(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r.URL.Path, "check")
	if !ok {
		http.NotFound(w, r)
		return
	}
	dev, err := a.Devices.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "device not found", 404)
		return
	}
	d := EnabledDevice{
		ID: dev.ID, Hostname: dev.DNSHostname, RecordType: dev.DNSRecordType,
		ResolverServer: dev.DNSResolverServer, ExpectedAnswer: dev.DNSExpectedAnswer,
	}
	var res Result
	if a.Poller != nil {
		res = a.Poller.Force(r.Context(), d)
	} else {
		res = Check(r.Context(), d.Hostname, d.RecordType, d.ResolverServer, d.ExpectedAnswer, 5*time.Second)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func pathID(path, suffix string) (string, bool) {
	path = strings.TrimPrefix(path, "/api/v1/dns/")
	path = strings.TrimSuffix(path, "/"+suffix)
	if path == "" {
		return "", false
	}
	return path, true
}
