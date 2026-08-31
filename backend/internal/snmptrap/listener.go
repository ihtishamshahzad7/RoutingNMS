package snmptrap

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// snmpTrapOID is the well-known varbind OID (SNMPv2-MIB::snmpTrapOID.0)
// carried in every v2c/v3 TRAP/INFORM PDU identifying which notification
// fired. v1 traps don't carry this -- their identity is Enterprise +
// Generic/SpecificTrap instead, handled separately below.
const snmpTrapOIDVarbind = ".1.3.6.1.6.3.1.1.4.1.0"

// Listener wraps gosnmp's TrapListener with storage against Repository and
// a fallback port so it can still run unprivileged (binding UDP/162
// requires root/CAP_NET_BIND_SERVICE on Linux, same constraint the syslog
// receiver has on TCP/UDP 514).
type Listener struct {
	Repo        Repository
	Communities []string // accepted v1/v2c community strings; empty = accept any
}

// ListenAndServe starts the trap listener on addr (e.g. ":162") until ctx
// is cancelled. Each received trap is parsed, matched against the current
// rule set and stored; storage/parse errors are logged and otherwise
// ignored so one malformed trap never brings down ingestion for the fleet.
func (l Listener) ListenAndServe(ctx context.Context, addr string) error {
	tl := gosnmp.NewTrapListener()
	tl.OnNewTrap = func(packet *gosnmp.SnmpPacket, src *net.UDPAddr) {
		l.handle(ctx, packet, src)
	}
	tl.Params = gosnmp.Default

	errCh := make(chan error, 1)
	go func() {
		errCh <- tl.Listen(addr)
	}()

	go func() {
		<-ctx.Done()
		tl.Close()
	}()

	log.Printf("snmp trap listener listening on %s (udp, v1/v2c/v3)", addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (l Listener) handle(ctx context.Context, packet *gosnmp.SnmpPacket, src *net.UDPAddr) {
	if packet == nil {
		return
	}
	if len(l.Communities) > 0 && packet.Version != gosnmp.Version3 {
		ok := false
		for _, c := range l.Communities {
			if c == packet.Community {
				ok = true
				break
			}
		}
		if !ok {
			log.Printf("snmptrap: dropped trap from %s: community not accepted", src.IP)
			return
		}
	}

	rt := ReceivedTrap{
		SourceIP: src.IP.String(),
		Version:  versionLabel(packet.Version),
	}

	for _, v := range packet.Variables {
		oid := strings.TrimPrefix(v.Name, ".")
		val := stringifyValue(v.Value)
		if v.Name == snmpTrapOIDVarbind || strings.TrimPrefix(v.Name, ".") == strings.TrimPrefix(snmpTrapOIDVarbind, ".") {
			rt.TrapOID = strings.TrimPrefix(strings.TrimSpace(val), ".")
		}
		rt.Varbinds = append(rt.Varbinds, Varbind{OID: oid, Type: v.Type.String(), Value: val})
	}

	// v1 traps don't carry an snmpTrapOID varbind; fall back to the
	// enterprise + generic/specific fields gosnmp exposes on the packet
	// itself for TRAP-PDU (v1).
	if rt.TrapOID == "" {
		enterprise := packet.Enterprise
		rt.EnterpriseOID = enterprise
		gt := packet.GenericTrap
		st := packet.SpecificTrap
		rt.GenericTrap = &gt
		rt.SpecificTrap = &st
		if enterprise != "" {
			rt.TrapOID = fmt.Sprintf("%s.0.%d", strings.TrimPrefix(enterprise, "."), st)
		}
	}

	storeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := l.Repo.Store(storeCtx, rt); err != nil {
		log.Printf("snmptrap: store failed (%s): %v", src.IP, err)
	}
}

func versionLabel(v gosnmp.SnmpVersion) string {
	switch v {
	case gosnmp.Version1:
		return "v1"
	case gosnmp.Version2c:
		return "v2c"
	case gosnmp.Version3:
		return "v3"
	default:
		return "unknown"
	}
}

func stringifyValue(v any) string {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// ListenWithFallback tries addr first (typically the privileged :162) and
// falls back to fallbackAddr (e.g. :1162) if binding fails, matching the
// SYSLOG_ADDR pattern already used elsewhere in this NMS for services that
// default to a privileged port.
func ListenWithFallback(ctx context.Context, l Listener, addr, fallbackAddr string) {
	if err := probeBind(addr); err != nil {
		log.Printf("snmptrap: cannot bind %s (%v), falling back to %s", addr, err, fallbackAddr)
		addr = fallbackAddr
	}
	if err := l.ListenAndServe(ctx, addr); err != nil && ctx.Err() == nil {
		log.Printf("snmptrap: listener stopped: %v", err)
	}
}

func probeBind(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	return conn.Close()
}
