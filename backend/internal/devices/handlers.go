package devices

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type Handler struct { Repo Repository }

type SNMPConfigRequest struct {
    Enabled bool `json:"enabled"`
    Version string `json:"version"`
    Community string `json:"community"`
    Username string `json:"username"`
    AuthProto string `json:"authProto"`
    AuthPass string `json:"authPass"`
    PrivProto string `json:"privProto"`
    PrivPass string `json:"privPass"`
    Port int `json:"port"`
    TimeoutMS int `json:"timeoutMs"`
}

func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { http.Error(w,"method not allowed",405); return }
    var in DeviceInput
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w,"invalid JSON",400); return }
    if err := ValidateRegistration(in); err != nil { http.Error(w,err.Error(),400); return }
    d, err := h.Repo.Create(r.Context(),in); if err != nil { http.Error(w,"failed to save device: "+err.Error(),500); return }
    w.Header().Set("Content-Type","application/json"); w.WriteHeader(http.StatusCreated); _=json.NewEncoder(w).Encode(d)
}

func (h Handler) List(w http.ResponseWriter, r *http.Request) {
    org := r.URL.Query().Get("organizationId"); if org == "" { http.Error(w,"organizationId is required",400); return }
    items, err := h.Repo.List(r.Context(),org); if err != nil { http.Error(w,"failed to load devices",500); return }
    w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(items)
}

func (h Handler) UpdateSNMP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPut { http.Error(w,"method not allowed",405); return }
    id := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
    id = strings.TrimSuffix(id, "/snmp")
    if id == "" { http.Error(w,"device ID is required",400); return }
    var req SNMPConfigRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w,"invalid JSON",400); return }
    if req.Port == 0 { req.Port = 161 }
    if req.TimeoutMS == 0 { req.TimeoutMS = 3000 }
    version := snmp.Version(strings.TrimSpace(strings.ToLower(req.Version)))
    if req.Enabled && version != snmp.V2c && version != snmp.V3 {
        http.Error(w,"SNMP version must be 2c or 3",400)
        return
    }
    req.Version = string(version)
    if err := h.Repo.UpdateSNMP(r.Context(), id, req); err != nil { http.Error(w,"failed to save SNMP configuration: "+err.Error(),500); return }
    w.Header().Set("Content-Type","application/json")
    _=json.NewEncoder(w).Encode(map[string]any{"status":"ok","deviceId":id,"snmpEnabled":req.Enabled,"snmpVersion":req.Version,"snmpPort":req.Port})
}
