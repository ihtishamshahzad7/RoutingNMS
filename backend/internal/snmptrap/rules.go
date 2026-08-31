// Package snmptrap implements SNMP v1/v2c/v3 trap reception and a small
// OID-pattern alert rule engine, modeled on the trap-rule approach used by
// several open-source NMS tools (rule = OID pattern -> severity + optional
// notification target) but implemented natively against this project's
// existing gosnmp dependency and Postgres schema.
package snmptrap

import "strings"

type Rule struct {
	ID               int64
	Name             string
	OIDPattern       string
	Severity         string
	Enabled          bool
	NotifyEmail      string
	NotifyWebhookURL string
}

// Match reports whether oid matches the rule's pattern. A pattern of ""
// or "*" matches everything. A pattern ending in ".*" or "*" matches by
// prefix (e.g. "1.3.6.1.4.1.9.*" matches any Cisco enterprise trap). Any
// other pattern must match the OID exactly.
func (r Rule) Match(oid string) bool {
	p := strings.TrimSpace(r.OIDPattern)
	if p == "" || p == "*" {
		return true
	}
	if strings.HasSuffix(p, "*") {
		prefix := strings.TrimSuffix(p, "*")
		prefix = strings.TrimSuffix(prefix, ".")
		return oid == prefix || strings.HasPrefix(oid, prefix+".")
	}
	return oid == p
}

// FirstMatch returns the first enabled rule (in the given order -- callers
// should pass rules ordered by specificity/priority, e.g. most specific
// pattern first) that matches oid, or false if none match.
func FirstMatch(rules []Rule, oid string) (Rule, bool) {
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if r.Match(oid) {
			return r, true
		}
	}
	return Rule{}, false
}
