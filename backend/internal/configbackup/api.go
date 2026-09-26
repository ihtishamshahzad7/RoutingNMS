package configbackup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ListAPI backs GET /api/v1/devices/{id}/config-backups (version metadata,
// newest first) and POST /api/v1/devices/{id}/config-backups (manual
// paste/upload of a config export) -- both session-authed.
type ListAPI struct{ Repo Repository }

func (a ListAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	deviceID := r.PathValue("id")

	switch r.Method {
	case http.MethodGet:
		items, err := a.Repo.List(ctx, deviceID)
		if err != nil {
			http.Error(w, "failed to load config backups", http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)

	case http.MethodPost:
		var req struct {
			ConfigText string `json:"configText"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.ConfigText) == "" {
			http.Error(w, "configText is required", http.StatusBadRequest)
			return
		}
		b, err := a.Repo.Store(ctx, deviceID, req.ConfigText, "manual")
		if err != nil {
			http.Error(w, "failed to store config backup", http.StatusInternalServerError)
			return
		}
		writeJSON(w, b)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// DetailAPI backs GET /api/v1/devices/{id}/config-backups/{backupId}
// (full config text of one version) and DELETE (remove one version) --
// both session-authed.
type DetailAPI struct{ Repo Repository }

func (a DetailAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	deviceID := r.PathValue("id")
	id, err := strconv.ParseInt(r.PathValue("backupId"), 10, 64)
	if err != nil {
		http.Error(w, "invalid backup id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		b, err := a.Repo.Get(ctx, deviceID, id)
		if err != nil {
			http.Error(w, "config backup not found", http.StatusNotFound)
			return
		}
		writeJSON(w, b)

	case http.MethodDelete:
		if err := a.Repo.Delete(ctx, deviceID, id); err != nil {
			http.Error(w, "failed to delete config backup", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// DiffAPI backs GET /api/v1/devices/{id}/config-backups/diff?from={id}&to={id}
// -- session-authed line diff between two stored versions.
type DiffAPI struct{ Repo Repository }

func (a DiffAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	deviceID := r.PathValue("id")

	fromID, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if err != nil {
		http.Error(w, "invalid or missing from", http.StatusBadRequest)
		return
	}
	toID, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if err != nil {
		http.Error(w, "invalid or missing to", http.StatusBadRequest)
		return
	}
	from, err := a.Repo.Get(ctx, deviceID, fromID)
	if err != nil {
		http.Error(w, "from version not found", http.StatusNotFound)
		return
	}
	to, err := a.Repo.Get(ctx, deviceID, toID)
	if err != nil {
		http.Error(w, "to version not found", http.StatusNotFound)
		return
	}
	writeJSON(w, struct {
		From  Backup     `json:"from"`
		To    Backup     `json:"to"`
		Lines []DiffLine `json:"lines"`
	}{From: from, To: to, Lines: LineDiff(from.ConfigText, to.ConfigText)})
}

// SetupAPI backs GET /api/v1/devices/{id}/config-backups/setup -- session-
// authed. Mirrors provisioning.PreviewAPI: shows an operator the exact
// RouterOS scheduler script that pushes a config export to PushAPI below on
// a recurring schedule, without the device having to ask for it.
type SetupAPI struct {
	Devices devices.Repository
	BaseURL string
	Token   string
}

type setupResponse struct {
	SchedulerScript string `json:"schedulerScript"`
}

func (a SetupAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	d, err := a.Devices.GetByID(ctx, r.PathValue("id"))
	if err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}
	if strings.TrimSpace(d.SerialNumber) == "" {
		http.Error(w, "device has no serial number recorded", http.StatusBadRequest)
		return
	}
	url := fmt.Sprintf("%s/api/v1/config-backup/routeros/%s?token=%s", a.BaseURL, d.SerialNumber, a.Token)
	script := fmt.Sprintf(`/system scheduler add name=routingnms-config-backup interval=1d on-event=(\
  ":local fname (\"cfg-backup-\" . [/system identity get name] . \".rsc\");" .\
  "/export file=\$fname;" .\
  ":delay 2;" .\
  "/tool fetch url=\"%s\" http-method=post src-path=(\$fname . \".rsc\") mode=https;"\
)`, url)
	writeJSON(w, setupResponse{SchedulerScript: script})
}

// PushAPI backs POST /api/v1/config-backup/routeros/{serial} -- the
// device-facing endpoint a RouterOS box's scheduled `/tool fetch
// http-method=post` job posts its `/export` output to. Same shared-token
// auth idiom as `provisioning.FetchAPI` (RouterOS has no session cookie),
// protected by a query-param token instead of the usual session middleware.
// The device must already be registered with this exact serial number.
type PushAPI struct {
	Devices devices.Repository
	Repo    Repository
	Token   string
}

func (a PushAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.Token == "" || r.URL.Query().Get("token") != a.Token {
		http.Error(w, "invalid or missing token", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	serial := strings.TrimSpace(r.PathValue("serial"))
	if serial == "" {
		http.Error(w, "serial number is required", http.StatusBadRequest)
		return
	}
	d, err := a.Devices.GetBySerial(ctx, serial)
	if err != nil {
		http.Error(w, "device not registered", http.StatusNotFound)
		return
	}
	body := make([]byte, 0, 8192)
	buf := make([]byte, 8192)
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	if len(body) == 0 {
		http.Error(w, "empty config body", http.StatusBadRequest)
		return
	}
	if _, err := a.Repo.Store(ctx, d.ID, string(body), "device-push"); err != nil {
		http.Error(w, "failed to store config backup", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
