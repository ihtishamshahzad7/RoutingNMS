package telnetcheck

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
)

// API exposes Telnet-check live status and on-demand checks, mirroring
// sshcheck.API's shape.
type API struct {
	Devices devices.Repository
	Poller  *Poller
}

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
	d := EnabledDevice{ID: dev.ID, Address: dev.Address, Port: dev.TelnetPort, BannerKeyword: dev.TelnetBannerKeyword}
	var res Result
	if a.Poller != nil {
		res = a.Poller.Force(r.Context(), d)
	} else {
		res = Check(r.Context(), d.Address, d.Port, d.BannerKeyword, 5*time.Second)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func pathID(path, suffix string) (string, bool) {
	path = strings.TrimPrefix(path, "/api/v1/telnet/")
	path = strings.TrimSuffix(path, "/"+suffix)
	if path == "" {
		return "", false
	}
	return path, true
}
