package devices

import (
	"context"
	"fmt"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// DiscoverInterfaces runs standard IF-MIB discovery and returns normalized
// interface inventory suitable for persistence. Vendor-specific interfaces
// can be added later through adapters without changing this contract.
func DiscoverInterfaces(ctx context.Context, in DeviceInput) ([]snmp.Interface, error) {
	if err := ValidateInput(in); err != nil { return nil, err }
	target := snmp.Target{ID: in.Name, Address: in.Address, Port: in.SNMPPort, Credentials: in.SNMP, Timeout: in.Timeout, Retries: 1}
	result, err := (snmp.Collector{}).Discover(ctx, target)
	if err != nil { return nil, fmt.Errorf("interface discovery: %w", err) }
	return result.Interfaces, nil
}
