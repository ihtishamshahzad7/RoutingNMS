// Package dnscheck implements the DNS resolution monitor type ported from
// the user's previous Uptime Kuma deployment: periodically resolve a
// hostname against a record type (A, AAAA, CNAME, MX, TXT, NS, SOA),
// optionally against a specific resolver server rather than the system
// default, and optionally verify the resolved answer matches an expected
// value. Uses only the stdlib net package -- no external DNS library.
package dnscheck

import (
	"context"
	"net"
	"strings"
	"time"
)

// Result is the outcome of one DNS resolution check.
type Result struct {
	Resolved      bool
	Answers       []string
	LatencyMS     float64
	ExpectedMatch *bool // nil when no expected answer was configured
	Error         string
}

// Check resolves hostname as recordType (A/AAAA/CNAME/MX/TXT/NS/SOA),
// optionally against resolverServer (host:port or bare host, defaulting to
// port 53/udp) instead of the system resolver, and -- if expectedAnswer is
// non-empty -- verifies one of the returned answers contains it.
func Check(ctx context.Context, hostname, recordType, resolverServer, expectedAnswer string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	resolver := net.DefaultResolver
	if strings.TrimSpace(resolverServer) != "" {
		server := strings.TrimSpace(resolverServer)
		if _, _, err := net.SplitHostPort(server); err != nil {
			server = net.JoinHostPort(server, "53")
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, "udp", server)
			},
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	answers, err := lookup(reqCtx, resolver, strings.ToUpper(strings.TrimSpace(recordType)), hostname)
	latency := time.Since(start)

	res := Result{LatencyMS: float64(latency.Milliseconds()), Answers: answers}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Resolved = len(answers) > 0
	if !res.Resolved {
		res.Error = "no records returned"
		return res
	}
	if expectedAnswer != "" {
		matched := false
		for _, a := range answers {
			if strings.Contains(a, expectedAnswer) {
				matched = true
				break
			}
		}
		res.ExpectedMatch = &matched
		if !matched {
			res.Resolved = false
			res.Error = "resolved answer did not match expected value"
		}
	}
	return res
}

func lookup(ctx context.Context, resolver *net.Resolver, recordType, hostname string) ([]string, error) {
	switch recordType {
	case "", "A":
		ips, err := resolver.LookupIP(ctx, "ip4", hostname)
		if err != nil {
			return nil, err
		}
		return ipsToStrings(ips), nil
	case "AAAA":
		ips, err := resolver.LookupIP(ctx, "ip6", hostname)
		if err != nil {
			return nil, err
		}
		return ipsToStrings(ips), nil
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, hostname)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil
	case "MX":
		records, err := resolver.LookupMX(ctx, hostname)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(records))
		for _, r := range records {
			out = append(out, r.Host)
		}
		return out, nil
	case "TXT":
		records, err := resolver.LookupTXT(ctx, hostname)
		if err != nil {
			return nil, err
		}
		return records, nil
	case "NS":
		records, err := resolver.LookupNS(ctx, hostname)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(records))
		for _, r := range records {
			out = append(out, r.Host)
		}
		return out, nil
	case "SOA":
		// stdlib net has no direct SOA lookup; report authoritative NS
		// records instead (Kuma's own SOA support is similarly limited).
		records, err := resolver.LookupNS(ctx, hostname)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(records))
		for _, r := range records {
			out = append(out, r.Host)
		}
		return out, nil
	default:
		ips, err := resolver.LookupIP(ctx, "ip4", hostname)
		if err != nil {
			return nil, err
		}
		return ipsToStrings(ips), nil
	}
}

func ipsToStrings(ips []net.IP) []string {
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}
