// Package mib implements a best-effort SMIv1/SMIv2 MIB text parser and an
// OID <-> name lookup store, plus a live "OID tester" that piggybacks on
// the existing internal/snmp Collector to fetch a real value from a device.
//
// This intentionally does not implement a full ASN.1/SMI compiler (that's a
// large undertaking even dedicated tools get wrong in edge cases). Instead
// it extracts every "<name> ... ::= { <parent> <subid> }" style assignment
// -- the pattern essentially every OBJECT-TYPE, OBJECT IDENTIFIER,
// MODULE-IDENTITY, OBJECT-IDENTITY and NOTIFICATION-TYPE definition in a
// real-world MIB follows -- and resolves the resulting name graph against a
// small set of well-known standard roots (iso, mib-2, enterprises, ...).
// Any name whose ancestry can't be resolved (e.g. it depends on a MIB that
// wasn't uploaded) is skipped rather than guessed at; the parse result
// reports how many definitions were resolved vs. skipped so operators can
// tell whether they need to also upload a dependency MIB.
package mib

import (
	"regexp"
	"strconv"
	"strings"
)

// wellKnownRoots seeds the standard SNMP OID tree so files that build on
// mib-2/enterprises/etc. (i.e. nearly all of them) resolve without also
// needing RFC1155-SMI/SNMPv2-SMI uploaded alongside them.
var wellKnownRoots = map[string]string{
	"iso":            "1",
	"org":            "1.3",
	"dod":            "1.3.6",
	"internet":       "1.3.6.1",
	"directory":      "1.3.6.1.1",
	"mgmt":           "1.3.6.1.2",
	"mib-2":          "1.3.6.1.2.1",
	"mib2":           "1.3.6.1.2.1",
	"transmission":   "1.3.6.1.2.1.10",
	"experimental":   "1.3.6.1.3",
	"private":        "1.3.6.1.4",
	"enterprises":    "1.3.6.1.4.1",
	"security":       "1.3.6.1.5",
	"snmpV2":         "1.3.6.1.6",
	"snmpModules":    "1.3.6.1.6.3",
	"snmpMIBObjects": "1.3.6.1.6.3.1",
}

// Object is a single resolved name -> OID mapping extracted from a MIB.
type Object struct {
	Name string
	OID  string
}

// assignRe matches "<name> <SMI macro keyword> ... ::= { <parent> <n> }".
// Requiring one of the known SMI macro keywords right after the name (as
// every real OBJECT-TYPE/OBJECT IDENTIFIER/etc. definition has) is what
// keeps this from misfiring on unrelated "::=" assignments elsewhere in the
// file, e.g. "<ModuleName> DEFINITIONS ::=" at the top of the file, which a
// looser "name ... ::= { }" pattern would incorrectly swallow together with
// the next real definition that follows it.
var assignRe = regexp.MustCompile(`(?ms)^([A-Za-z][A-Za-z0-9-]*)\s+(?:OBJECT-TYPE|OBJECT\s+IDENTIFIER|MODULE-IDENTITY|OBJECT-IDENTITY|NOTIFICATION-TYPE|MODULE-COMPLIANCE|OBJECT-GROUP|NOTIFICATION-GROUP)\b[\s\S]*?::=\s*\{\s*([A-Za-z][A-Za-z0-9-]*|\d+)\s+(\d+)\s*\}`)

// ParseResult is the outcome of parsing one MIB file.
type ParseResult struct {
	ModuleName string
	Objects    []Object // resolved name -> OID
	Skipped    int      // definitions whose ancestry couldn't be resolved
}

// Parse extracts and resolves OID assignments from raw MIB text.
func Parse(text string) ParseResult {
	moduleName := detectModuleName(text)
	clean := stripComments(text)

	type pending struct {
		name, parent string
		subid        int
	}
	var defs []pending
	for _, m := range assignRe.FindAllStringSubmatch(clean, -1) {
		name, parent, subidStr := m[1], m[2], m[3]
		subid, err := strconv.Atoi(subidStr)
		if err != nil {
			continue
		}
		defs = append(defs, pending{name: name, parent: parent, subid: subid})
	}

	resolved := map[string]string{}
	for k, v := range wellKnownRoots {
		resolved[k] = v
	}

	// Iterate to a fixpoint: each pass resolves any definition whose parent
	// is now known, since MIBs commonly define children before ancestors
	// appear textually (or vice versa) and refer to sibling definitions.
	remaining := defs
	for {
		var next []pending
		progress := false
		for _, d := range remaining {
			parentOID := d.parent
			if !isNumeric(parentOID) {
				oid, ok := resolved[d.parent]
				if !ok {
					next = append(next, d)
					continue
				}
				parentOID = oid
			}
			resolved[d.name] = parentOID + "." + strconv.Itoa(d.subid)
			progress = true
		}
		remaining = next
		if !progress || len(remaining) == 0 {
			break
		}
	}

	seen := map[string]bool{}
	var objects []Object
	for _, d := range defs {
		if oid, ok := resolved[d.name]; ok && !seen[d.name] {
			seen[d.name] = true
			objects = append(objects, Object{Name: d.name, OID: oid})
		}
	}

	return ParseResult{ModuleName: moduleName, Objects: objects, Skipped: len(remaining)}
}

var moduleRe = regexp.MustCompile(`(?m)^([A-Za-z][A-Za-z0-9-]*)\s+DEFINITIONS\s*(::=)?\s*$`)

func detectModuleName(text string) string {
	if m := moduleRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stripComments removes SMI "--" line comments (to end of line) so they
// can't accidentally contain something that looks like an assignment.
// SMI comments run to end-of-line or the next "--", whichever the source
// text actually uses; treating them as line comments is the safe
// approximation nearly every real-world MIB satisfies.
func stripComments(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}
