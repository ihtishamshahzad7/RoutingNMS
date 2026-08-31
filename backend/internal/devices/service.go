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
	SerialNumber string
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

// ValidateRegistration performs the cheap validation needed to save a device
// record (the "Add device" flow: POST /api/v1/devices). It deliberately does
// not require SNMP credentials or a timeout -- those are only meaningful for
// an active SNMP probe (ValidateInput/TestSNMP below), and requiring them
// here made every registration fail with "timeout must be positive" since
// the registration form never collects them.
func ValidateRegistration(in DeviceInput) error {
	if strings.TrimSpace(in.OrganizationID) == "" { return fmt.Errorf("organization ID is required") }
	if strings.TrimSpace(in.Name) == "" { return fmt.Errorf("device name is required") }
	if net.ParseIP(strings.TrimSpace(in.Address)) == nil { return fmt.Errorf("address must be a valid IP address") }
	return nil
}

// ValidateInput performs cheap validation before any network operation
// (an active SNMP probe -- see TestSNMP). Unlike ValidateRegistration, this
// requires SNMP credentials and a timeout, since those are used to actually
// reach the device over the network.
func ValidateInput(in DeviceInput) error {
	if err := ValidateRegistration(in); err != nil { return err }
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
