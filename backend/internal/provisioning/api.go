package provisioning

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

// TemplatesAPI backs GET/POST /api/v1/provisioning/templates and
// GET/PUT/DELETE /api/v1/provisioning/templates/{id} -- session-authed CRUD
// for the reusable script templates admins assign to router devices.
type TemplatesAPI struct{ Repo Repository }

func (a TemplatesAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		items, err := a.Repo.List(ctx)
		if err != nil {
			http.Error(w, "failed to load templates", http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)

	case r.Method == http.MethodPost && idStr == "":
		var req struct {
			Name       string `json:"name"`
			ScriptBody string `json:"scriptBody"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		t, err := a.Repo.Create(ctx, req.Name, req.ScriptBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, t)

	case r.Method == http.MethodGet && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid template id", http.StatusBadRequest)
			return
		}
		t, err := a.Repo.Get(ctx, id)
		if err != nil {
			http.Error(w, "template not found", http.StatusNotFound)
			return
		}
		writeJSON(w, t)

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid template id", http.StatusBadRequest)
			return
		}
		var req struct {
			Name       string `json:"name"`
			ScriptBody string `json:"scriptBody"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		t, err := a.Repo.Update(ctx, id, req.Name, req.ScriptBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, t)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid template id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete template", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// AssignAPI backs PUT /api/v1/devices/{id}/provisioning -- session-authed,
// assigns (or clears) which template a device will receive.
type AssignAPI struct {
	Templates Repository
	Devices   devices.Repository
}

func (a AssignAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	deviceID := r.PathValue("id")
	var req struct {
		TemplateID *int64 `json:"templateId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := a.Devices.UpdateProvisioning(ctx, deviceID, req.TemplateID); err != nil {
		http.Error(w, "failed to update provisioning assignment", http.StatusInternalServerError)
		return
	}
	d, err := a.Devices.GetByID(ctx, deviceID)
	if err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

// PreviewAPI backs GET /api/v1/devices/{id}/provisioning/preview --
// session-authed. Shows an operator the rendered script, the derived
// password, and the exact RouterOS fetch command, without the router
// itself having to ask for it.
type PreviewAPI struct {
	Templates Repository
	Devices   devices.Repository
	Salt      string
	BaseURL   string
	Token     string
}

type previewResponse struct {
	RenderedScript string `json:"renderedScript"`
	Password       string `json:"password"`
	FetchCommand   string `json:"fetchCommand"`
}

func (a PreviewAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if d.ProvisioningTemplateID == nil {
		http.Error(w, "device has no provisioning template assigned", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(d.SerialNumber) == "" {
		http.Error(w, "device has no serial number recorded", http.StatusBadRequest)
		return
	}
	tpl, err := a.Templates.Get(ctx, *d.ProvisioningTemplateID)
	if err != nil {
		http.Error(w, "assigned template not found", http.StatusInternalServerError)
		return
	}
	password := DerivePassword(d.SerialNumber, a.Salt)
	rendered, err := Render(tpl.ScriptBody, RenderData{Hostname: d.Name, Address: d.Address, Password: password})
	if err != nil {
		http.Error(w, "failed to render template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fetchCmd := fmt.Sprintf(`/tool fetch url="%s/api/v1/provision/routeros/%s?token=%s" mode=https dst-path=provision.rsc`, a.BaseURL, d.SerialNumber, a.Token)
	writeJSON(w, previewResponse{RenderedScript: rendered, Password: password, FetchCommand: fetchCmd})
}

// FetchAPI backs GET /api/v1/provision/routeros/{serial} -- the
// device-facing endpoint a RouterOS box hits with `/tool fetch`. RouterOS
// has no session cookie, so this is protected by a shared token query
// param (PROVISION_TOKEN) instead of the usual auth middleware. The device
// must already be pre-registered with this exact serial number and have a
// template assigned -- there is no auto-registration of unknown devices.
type FetchAPI struct {
	Templates Repository
	Devices   devices.Repository
	Salt      string
	Token     string
}

func (a FetchAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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
	if d.ProvisioningTemplateID == nil {
		http.Error(w, "device has no provisioning template assigned", http.StatusNotFound)
		return
	}
	tpl, err := a.Templates.Get(ctx, *d.ProvisioningTemplateID)
	if err != nil {
		http.Error(w, "assigned template not found", http.StatusInternalServerError)
		return
	}
	password := DerivePassword(d.SerialNumber, a.Salt)
	rendered, err := Render(tpl.ScriptBody, RenderData{Hostname: d.Name, Address: d.Address, Password: password})
	if err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		return
	}
	_ = a.Devices.TouchProvisioned(ctx, d.ID)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(rendered))
}
