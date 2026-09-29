package hostmetrics

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Reading is one poll cycle's CPU/memory result. Either field can be nil on
// its own -- e.g. a generic device that has hrProcessorLoad but no
// hrStorage RAM row -- so a partial reading still records what it got
// rather than discarding the whole poll.
type Reading struct {
	CPUPercent    *float64
	MemoryPercent *float64
}

// Poll dispatches to the right OID set for vendor (as stored on
// devices.vendor), falling back to generic HOST-RESOURCES-MIB for anything
// that isn't recognized as MikroTik or Cisco.
func Poll(ctx context.Context, collector snmp.Collector, target snmp.Target, vendor string) (Reading, error) {
	v := strings.ToLower(vendor)
	switch {
	case strings.Contains(v, "mikrotik") || strings.Contains(v, "routeros"):
		return pollMikroTik(ctx, collector, target)
	case strings.Contains(v, "cisco"):
		return pollCiscoIOS(ctx, collector, target)
	default:
		return pollGeneric(ctx, collector, target)
	}
}

func pollGeneric(ctx context.Context, collector snmp.Collector, target snmp.Target) (Reading, error) {
	var reading Reading

	if values, err := collector.Walk(ctx, target, HrProcessorLoadOID); err == nil && len(values) > 0 {
		sum, n := 0.0, 0
		for _, v := range values {
			sum += numberValue(v.Value)
			n++
		}
		if n > 0 {
			avg := sum / float64(n)
			reading.CPUPercent = &avg
		}
	}

	// hrStorageTable: match the row whose hrStorageType is the RAM entry,
	// then read size/used at that same row index. Three separate walks
	// correlated by the trailing table index (RFC 2790 tables are all
	// indexed the same way: <base oid>.<index>).
	typeRows, errType := collector.Walk(ctx, target, HrStorageTypeOID)
	sizeRows, errSize := collector.Walk(ctx, target, HrStorageSizeOID)
	usedRows, errUsed := collector.Walk(ctx, target, HrStorageUsedOID)
	if errType == nil && errSize == nil && errUsed == nil {
		sizeByIdx := map[string]float64{}
		for _, v := range sizeRows {
			if idx, ok := trailingIndex(v.OID, HrStorageSizeOID); ok {
				sizeByIdx[idx] = numberValue(v.Value)
			}
		}
		usedByIdx := map[string]float64{}
		for _, v := range usedRows {
			if idx, ok := trailingIndex(v.OID, HrStorageUsedOID); ok {
				usedByIdx[idx] = numberValue(v.Value)
			}
		}
		for _, v := range typeRows {
			idx, ok := trailingIndex(v.OID, HrStorageTypeOID)
			if !ok || !oidEquals(v.Value, HrStorageRAMOID) {
				continue
			}
			size, sizeOK := sizeByIdx[idx]
			used, usedOK := usedByIdx[idx]
			if sizeOK && usedOK && size > 0 {
				pct := used / size * 100
				reading.MemoryPercent = &pct
			}
			break
		}
	}

	if reading.CPUPercent == nil && reading.MemoryPercent == nil {
		return reading, fmt.Errorf("host-resources-mib: no usable CPU or memory data")
	}
	return reading, nil
}

func pollMikroTik(ctx context.Context, collector snmp.Collector, target snmp.Target) (Reading, error) {
	var reading Reading
	values, err := collector.Get(ctx, target, []string{MikroTikCPUOID, MikroTikMemTotalOID, MikroTikMemFreeOID})
	if err != nil {
		return reading, fmt.Errorf("mikrotik host metrics poll: %w", err)
	}
	var total, free float64
	var haveTotal, haveFree bool
	for _, v := range values {
		switch {
		case strings.HasSuffix(v.OID, strings.TrimPrefix(MikroTikCPUOID, ".")):
			cpu := numberValue(v.Value)
			reading.CPUPercent = &cpu
		case strings.HasSuffix(v.OID, strings.TrimPrefix(MikroTikMemTotalOID, ".")):
			total = numberValue(v.Value)
			haveTotal = true
		case strings.HasSuffix(v.OID, strings.TrimPrefix(MikroTikMemFreeOID, ".")):
			free = numberValue(v.Value)
			haveFree = true
		}
	}
	if haveTotal && haveFree && total > 0 {
		pct := (total - free) / total * 100
		reading.MemoryPercent = &pct
	}
	return reading, nil
}

func pollCiscoIOS(ctx context.Context, collector snmp.Collector, target snmp.Target) (Reading, error) {
	var reading Reading

	if values, err := collector.Walk(ctx, target, CiscoCPUTotal5MinRevOID); err == nil && len(values) > 0 {
		sum, n := 0.0, 0
		for _, v := range values {
			sum += numberValue(v.Value)
			n++
		}
		if n > 0 {
			avg := sum / float64(n)
			reading.CPUPercent = &avg
		}
	}

	nameRows, errName := collector.Walk(ctx, target, CiscoMemPoolNameOID)
	usedRows, errUsed := collector.Walk(ctx, target, CiscoMemPoolUsedOID)
	freeRows, errFree := collector.Walk(ctx, target, CiscoMemPoolFreeOID)
	if errName == nil && errUsed == nil && errFree == nil {
		usedByIdx := map[string]float64{}
		for _, v := range usedRows {
			if idx, ok := trailingIndex(v.OID, CiscoMemPoolUsedOID); ok {
				usedByIdx[idx] = numberValue(v.Value)
			}
		}
		freeByIdx := map[string]float64{}
		for _, v := range freeRows {
			if idx, ok := trailingIndex(v.OID, CiscoMemPoolFreeOID); ok {
				freeByIdx[idx] = numberValue(v.Value)
			}
		}
		// Prefer the pool named "Processor" (the platform-wide pool on
		// classic IOS); if no pool matches by name, fall back to the
		// first pool reported rather than skipping memory entirely.
		var fallbackIdx string
		haveFallback := false
		for _, v := range nameRows {
			idx, ok := trailingIndex(v.OID, CiscoMemPoolNameOID)
			if !ok {
				continue
			}
			if !haveFallback {
				fallbackIdx, haveFallback = idx, true
			}
			if strings.Contains(strings.ToLower(stringValue(v.Value)), "processor") {
				fallbackIdx, haveFallback = idx, true
				break
			}
		}
		if haveFallback {
			used, usedOK := usedByIdx[fallbackIdx]
			free, freeOK := freeByIdx[fallbackIdx]
			if usedOK && freeOK && used+free > 0 {
				pct := used / (used + free) * 100
				reading.MemoryPercent = &pct
			}
		}
	}

	if reading.CPUPercent == nil && reading.MemoryPercent == nil {
		return reading, fmt.Errorf("cisco host metrics poll: no usable CPU or memory data")
	}
	return reading, nil
}

// trailingIndex returns the table index suffix of a walked OID relative to
// its column's base OID (e.g. ".1.3.6.1.2.1.25.2.3.1.5.1" with base
// "1.3.6.1.2.1.25.2.3.1.5" -> "1"), tolerating a leading '.' on either
// side the way gosnmp's PDU names do.
func trailingIndex(oid, base string) (string, bool) {
	o := strings.TrimPrefix(oid, ".")
	b := strings.TrimPrefix(base, ".")
	if !strings.HasPrefix(o, b+".") {
		return "", false
	}
	idx := strings.TrimPrefix(o[len(b):], ".")
	if idx == "" {
		return "", false
	}
	return idx, true
}

// oidEquals reports whether a walked value (an OID, gosnmp returns
// hrStorageType as an object identifier) matches want, ignoring a leading
// '.'.
func oidEquals(v any, want string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	return strings.TrimPrefix(s, ".") == strings.TrimPrefix(want, ".")
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return ""
	}
}

func numberValue(v any) float64 {
	switch x := v.(type) {
	case uint64:
		return float64(x)
	case uint32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		if x < 0 {
			return 0
		}
		return float64(x)
	case float64:
		return x
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	case []byte:
		n, _ := strconv.ParseFloat(strings.TrimSpace(string(x)), 64)
		return n
	default:
		return 0
	}
}
