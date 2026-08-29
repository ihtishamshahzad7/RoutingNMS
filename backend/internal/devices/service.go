package devices

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type DeviceInput struct {
	OrganizationID string
	Name string
	Address string
	DeviceType string
	Vendor string
	SNMP snmp.Credentials
	SNMPPort uint16
	Timeout time.Duration
}

type TestResult struct {
	Reachable bool `json:"reachable"`
	Address string `json:"address"`
	SystemName string `json:"systemName,omitempty"`
	SysDescr string `json:"sysDescr,omitempty"`
	InterfaceCount int `json:"interfaceCount"`
	Error string `json:"error,omitempty"`
}

// ValidateInput performs cheap validation before any network operation.
func ValidateInput(in DeviceInput) error {
	if strings.TrimSpace(in.OrganizationID) == "" { return fmt.Errorf("organization ID is required") }
	if strings.TrimSpace(in.Name) == "" { return fmt.Errorf("device name is required") }
	if net.ParseIP(strings.TrimSpace(in.Address)) == nil { return fmt.Errorf("address must be a valid IP address") }
	if in.SNMPPort == 0 { in.SNMPPort = 161 }
	if in.Timeout <= 0 { return fmt.Errorf("timeout must be positive") }
	if in.SNMP.Version != snmp.V2c && in.SNMP.Version != snmp.V3 { return fmt.Errorf("SNMP version must be 2c or 3") }
	return nil
}

func TestSNMP(ctx context.Context, in DeviceInput) (TestResult, error) {
	if err := ValidateInput(in); err != nil { return TestResult{}, err }
	target := snmp.Target{ID: in.Name, Address: in.Address, Port: in.SNMPPort, Credentials: in.SNMP, Timeout: in.Timeout, Retries: 1}
	result := TestResult{Address: in.Address}
	discovery, err := (snmp.Collector{}).Discover(ctx, target)
	if err != nil { result.Error = err.Error(); return result, err }
	result.Reachable = true
	result.SystemName = discovery.SystemName
	result.SysDescr = discovery.SysDescr
	result.InterfaceCount = len(discovery.Interfaces)
	return result, nil
}
