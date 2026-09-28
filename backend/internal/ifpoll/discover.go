package ifpoll

import (
	"context"
	"fmt"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// WalkFunc polls one device's full interface table. Swappable for tests.
type WalkFunc func(ctx context.Context, collector snmp.Collector, target snmp.Target) ([]PortState, error)

// walkInterfaces polls IF-MIB for every interface on target: ifDescr,
// ifAdminStatus, ifOperStatus, speed (ifHighSpeed preferred, ifSpeed
// fallback), plus in/out octet counters.
//
// Octet counters prefer the 64-bit ifXTable HC counters (ifHCInOctets/
// ifHCOutOctets); when a walk of those OIDs returns nothing at all --
// some older switches/OLTs/media converters don't implement ifXTable --
// this falls back to the 32-bit ifInOctets/ifOutOctets instead, and the
// caller is told which width was used (CounterWidth) so rate math applies
// the right wraparound modulus (2^32 vs 2^64) later.
func walkInterfaces(ctx context.Context, collector snmp.Collector, target snmp.Target) ([]PortState, error) {
	client, err := collector.Connect(ctx, target)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	ports := map[int64]*PortState{}
	get := func(idx int64) *PortState {
		p := ports[idx]
		if p == nil {
			p = &PortState{IfIndex: idx}
			ports[idx] = p
		}
		return p
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := walkOID(client, snmp.IfDescrOID, func(idx int64, v any) { get(idx).Description = fmt.Sprint(v) }); err != nil {
		return nil, fmt.Errorf("walk ifDescr: %w", err)
	}
	if err := walkOID(client, snmp.IfAdminStatusOID, func(idx int64, v any) { get(idx).AdminUp = uint64Value(v) == 1 }); err != nil {
		return nil, fmt.Errorf("walk ifAdminStatus: %w", err)
	}
	if err := walkOID(client, snmp.IfOperStatusOID, func(idx int64, v any) { get(idx).OperUp = uint64Value(v) == 1 }); err != nil {
		return nil, fmt.Errorf("walk ifOperStatus: %w", err)
	}
	// Speed is best-effort: some virtual/tunnel interfaces don't report it
	// meaningfully, but a failed walk here shouldn't abort the whole poll
	// (state/status matter more than the speed label). Prefer ifHighSpeed
	// (ifXTable, reported in Mbps) over the 32-bit ifSpeed -- ifSpeed caps
	// out/misreports on links at or above ~4.295 Gbps (RFC 2863 says
	// ifSpeed should report the max uint32 value in that case, which
	// ifHighSpeed exists specifically to fix), so any device that answers
	// it gives us the accurate figure. Fall back to ifSpeed only when a
	// port's ifHighSpeed is missing or reported as zero.
	_ = walkOID(client, snmp.IfSpeedOID, func(idx int64, v any) { get(idx).SpeedBps = uint64Value(v) })
	_ = walkOID(client, snmp.IfHighSpeedOID, func(idx int64, v any) {
		mbps := uint64Value(v)
		if mbps > 0 {
			get(idx).SpeedBps = mbps * 1_000_000
		}
	})

	hcIn := map[int64]uint64{}
	_ = walkOID(client, snmp.IfHCInOctetsOID, func(idx int64, v any) { hcIn[idx] = uint64Value(v) })
	hcOut := map[int64]uint64{}
	_ = walkOID(client, snmp.IfHCOutOctetsOID, func(idx int64, v any) { hcOut[idx] = uint64Value(v) })

	if len(hcIn) > 0 || len(hcOut) > 0 {
		for idx, v := range hcIn {
			p := get(idx)
			p.InOctets = v
			p.CounterWidth = 64
		}
		for idx, v := range hcOut {
			p := get(idx)
			p.OutOctets = v
			if p.CounterWidth == 0 {
				p.CounterWidth = 64
			}
		}
	} else {
		in32 := map[int64]uint64{}
		_ = walkOID(client, snmp.IfInOctetsOID, func(idx int64, v any) { in32[idx] = uint64Value(v) })
		out32 := map[int64]uint64{}
		_ = walkOID(client, snmp.IfOutOctetsOID, func(idx int64, v any) { out32[idx] = uint64Value(v) })
		for idx, v := range in32 {
			p := get(idx)
			p.InOctets = v
			p.CounterWidth = 32
		}
		for idx, v := range out32 {
			p := get(idx)
			p.OutOctets = v
			if p.CounterWidth == 0 {
				p.CounterWidth = 32
			}
		}
	}

	out := make([]PortState, 0, len(ports))
	for _, p := range ports {
		if p.CounterWidth == 0 {
			p.CounterWidth = 64
		}
		out = append(out, *p)
	}
	return out, nil
}
