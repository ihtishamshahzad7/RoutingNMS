package devices

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type DiscoveryHandler struct { Repo Repository }

func deviceIDFromPath(path string) string {
	id := strings.TrimPrefix(path, "/api/v1/devices/")
	id = strings.TrimSuffix(id, "/discover")
	id = strings.TrimSuffix(id, "/interfaces")
	return strings.Trim(id, "/")
}

func (h DiscoveryHandler) Discover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w,"method not allowed",405); return }
	id := deviceIDFromPath(r.URL.Path)
	if id == "" { http.Error(w,"device ID is required",400); return }
	input, err := h.Repo.DiscoveryTarget(r.Context(), id)
	if err != nil { http.Error(w, err.Error(), 400); return }
	result, err := (snmp.Collector{}).Discover(r.Context(), snmp.Target{ID:input.Name,Address:input.Address,Port:input.SNMPPort,Credentials:input.SNMP,Timeout:input.Timeout,Retries:1})
	if err != nil { w.Header().Set("Content-Type","application/json"); w.WriteHeader(502); _=json.NewEncoder(w).Encode(map[string]any{"error":err.Error(),"deviceId":id}); return }
	if err := SaveInterfaces(r.Context(), h.Repo.DB, id, result.Interfaces); err != nil { http.Error(w,"discovery succeeded but interface inventory could not be saved: "+err.Error(),500); return }
	w.Header().Set("Content-Type","application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"deviceId":id,"systemName":result.SystemName,"sysDescr":result.SysDescr,"interfaceCount":len(result.Interfaces),"interfaces":result.Interfaces})
}

func (h DiscoveryHandler) Interfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w,"method not allowed",405); return }
	id := deviceIDFromPath(r.URL.Path)
	if id == "" { http.Error(w,"device ID is required",400); return }
	items, err := h.Repo.ListInterfaces(r.Context(), id)
	if err != nil { http.Error(w,"failed to load interface inventory: "+err.Error(),500); return }
	w.Header().Set("Content-Type","application/json")
	_ = json.NewEncoder(w).Encode(items)
}
