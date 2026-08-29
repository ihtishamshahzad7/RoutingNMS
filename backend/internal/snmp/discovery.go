package snmp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

// DiscoveryResult is the normalized inventory returned after an SNMP walk.
type DiscoveryResult struct {
	SystemName string     `json:"systemName,omitempty"`
	SysDescr   string     `json:"sysDescr,omitempty"`
	SysObject  string     `json:"sysObjectId,omitempty"`
	Uptime     uint64     `json:"uptime,omitempty"`
	Interfaces []Interface `json:"interfaces"`
}

type Interface struct {
	Index       string `json:"index"`
	Description string `json:"description,omitempty"`
	AdminUp     bool   `json:"adminUp"`
	OperUp      bool   `json:"operUp"`
	InOctets    uint64 `json:"inOctets"`
	OutOctets   uint64 `json:"outOctets"`
	InErrors    uint64 `json:"inErrors"`
	OutErrors   uint64 `json:"outErrors"`
}

func (Collector) Discover(ctx context.Context, target Target) (DiscoveryResult, error) {
	client, err := (Collector{}).Connect(ctx, target)
	if err != nil { return DiscoveryResult{}, err }
	defer client.Conn.Close()

	result := DiscoveryResult{}
	values, err := client.Get([]string{SysNameOID, SysDescrOID, SysObjectIDOID, SysUpTimeOID})
	if err != nil { return result, fmt.Errorf("system discovery: %w", err) }
	for _, v := range values.Variables {
		switch v.Name {
		case SysNameOID: result.SystemName = fmt.Sprint(v.Value)
		case SysDescrOID: result.SysDescr = fmt.Sprint(v.Value)
		case SysObjectIDOID: result.SysObject = fmt.Sprint(v.Value)
		case SysUpTimeOID: result.Uptime = uint64Value(v.Value)
		}
	}

	interfaces := map[string]*Interface{}
	walk := func(oid string, apply func(*Interface, any)) error {
		return client.Walk(oid, func(variable gosnmp.SnmpPDU) error {
			index := strings.TrimPrefix(variable.Name, oid+".")
			item := interfaces[index]
			if item == nil { item = &Interface{Index: index}; interfaces[index] = item }
			apply(item, variable.Value)
			return nil
		})
	}

	if err := walk(IfDescrOID, func(i *Interface, v any) { i.Description = fmt.Sprint(v) }); err != nil { return result, err }
	if err := walk(IfAdminStatusOID, func(i *Interface, v any) { i.AdminUp = uint64Value(v) == 1 }); err != nil { return result, err }
	if err := walk(IfOperStatusOID, func(i *Interface, v any) { i.OperUp = uint64Value(v) == 1 }); err != nil { return result, err }
	if err := walk(IfHCInOctetsOID, func(i *Interface, v any) { i.InOctets = uint64Value(v) }); err != nil { return result, err }
	if err := walk(IfHCOutOctetsOID, func(i *Interface, v any) { i.OutOctets = uint64Value(v) }); err != nil { return result, err }
	if err := walk(IfInErrorsOID, func(i *Interface, v any) { i.InErrors = uint64Value(v) }); err != nil { return result, err }
	if err := walk(IfOutErrorsOID, func(i *Interface, v any) { i.OutErrors = uint64Value(v) }); err != nil { return result, err }

	for _, item := range interfaces { result.Interfaces = append(result.Interfaces, *item) }
	return result, nil
}

func uint64Value(v any) uint64 {
	switch x := v.(type) {
	case uint64: return x
	case uint32: return uint64(x)
	case int: return uint64(x)
	case string:
		n, _ := strconv.ParseUint(x, 10, 64); return n
	case []byte:
		var n uint64; for _, b := range x { n = n<<8 | uint64(b) }; return n
	default: return 0
	}
}

var _ = time.Second
