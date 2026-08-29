package devices

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type TestHandler struct{}

func (TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var req struct {
		OrganizationID string `json:"organizationId"`
		Name string `json:"name"`
		Address string `json:"address"`
		DeviceType string `json:"deviceType"`
		Vendor string `json:"vendor"`
		SNMPPort uint16 `json:"snmpPort"`
		TimeoutMS int `json:"timeoutMs"`
		SNMP struct { Version string `json:"version"`; Community string `json:"community"`; Username string `json:"username"`; AuthProto string `json:"authProto"`; AuthPass string `json:"authPass"`; PrivProto string `json:"privProto"`; PrivPass string `json:"privPass"` } `json:"snmp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "invalid JSON", http.StatusBadRequest); return }
	version := snmp.V2c; if req.SNMP.Version == "3" { version = snmp.V3 }
	result, err := TestSNMP(r.Context(), DeviceInput{OrganizationID:req.OrganizationID, Name:req.Name, Address:req.Address, DeviceType:req.DeviceType, Vendor:req.Vendor, SNMPPort:req.SNMPPort, Timeout:time.Duration(req.TimeoutMS)*time.Millisecond, SNMP:snmp.Credentials{Version:version, Community:req.SNMP.Community, Username:req.SNMP.Username, AuthProto:req.SNMP.AuthProto, AuthPass:req.SNMP.AuthPass, PrivProto:req.SNMP.PrivProto, PrivPass:req.SNMP.PrivPass}})
	status := http.StatusOK; if err != nil { status = http.StatusBadGateway }
	w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(result)
}
