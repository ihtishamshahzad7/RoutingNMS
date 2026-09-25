package workspacetopology

// Link validation: closes the "link validation" follow-up flagged as open
// since feature 32's writeup. A canvas link's sourcePort/targetPort are
// free-text strings (typed by hand, or copied from topology-links' SNMP
// suggestion <datalist> -- feature 24) that nothing has ever checked
// against reality. This runs a real SNMP interface walk against both
// ends' linked real devices (reusing the exact same
// devices.Repository.DiscoveryTarget + snmp.Collector.Discover call the
// existing POST /api/v1/devices/{id}/discover endpoint already makes --
// no new SNMP/OID code) and persists whether each link's ports actually
// exist and are up.

import (
	"context"
	"strings"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type LinkValidator struct {
	Repo    Repository
	Devices devices.Repository
}

// ValidateGroup runs a validation pass over every link in groupID,
// persists each result via Repository.SetLinkValidation, and returns the
// group's links with their freshly-set validation fields.
func (v LinkValidator) ValidateGroup(ctx context.Context, groupID int64) ([]Link, error) {
	canvasDevices, err := v.Repo.DevicesOf(ctx, groupID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Device, len(canvasDevices))
	for _, d := range canvasDevices {
		byID[d.ID] = d
	}

	links, err := v.Repo.LinksOf(ctx, groupID)
	if err != nil {
		return nil, err
	}

	// Cache one discovery walk per real linkedDeviceId within this pass --
	// a device can be an endpoint of several canvas links in the same
	// group, and each walk is a live SNMP round trip.
	discoveries := map[string]snmp.DiscoveryResult{}
	discoverErrs := map[string]error{}
	discover := func(realID string) (snmp.DiscoveryResult, error) {
		if res, ok := discoveries[realID]; ok {
			return res, nil
		}
		if e, ok := discoverErrs[realID]; ok {
			return snmp.DiscoveryResult{}, e
		}
		input, err := v.Devices.DiscoveryTarget(ctx, realID)
		if err != nil {
			discoverErrs[realID] = err
			return snmp.DiscoveryResult{}, err
		}
		res, err := (snmp.Collector{}).Discover(ctx, snmp.Target{
			ID: input.Name, Address: input.Address, Port: input.SNMPPort,
			Credentials: input.SNMP, Timeout: input.Timeout, Retries: 1,
		})
		if err != nil {
			discoverErrs[realID] = err
			return snmp.DiscoveryResult{}, err
		}
		discoveries[realID] = res
		return res, nil
	}

	for i := range links {
		l := &links[i]
		src, srcOK := byID[l.SourceID]
		dst, dstOK := byID[l.TargetID]
		if !srcOK || !dstOK || src.LinkedDeviceID == nil || dst.LinkedDeviceID == nil ||
			strings.TrimSpace(l.SourcePort) == "" || strings.TrimSpace(l.TargetPort) == "" {
			l.ValidationStatus = "unverified"
			l.ValidationDetail = "one or both ends aren't linked to a real device, or a port name is blank"
			_ = v.Repo.SetLinkValidation(ctx, l.ID, l.ValidationStatus, l.ValidationDetail)
			continue
		}

		srcResult, srcErr := discover(*src.LinkedDeviceID)
		dstResult, dstErr := discover(*dst.LinkedDeviceID)
		if srcErr != nil || dstErr != nil {
			l.ValidationStatus = "error"
			switch {
			case srcErr != nil && dstErr != nil:
				l.ValidationDetail = "both ends unreachable: " + srcErr.Error() + "; " + dstErr.Error()
			case srcErr != nil:
				l.ValidationDetail = "source device: " + srcErr.Error()
			default:
				l.ValidationDetail = "target device: " + dstErr.Error()
			}
			_ = v.Repo.SetLinkValidation(ctx, l.ID, l.ValidationStatus, l.ValidationDetail)
			continue
		}

		srcIface, srcFound := findInterface(srcResult, l.SourcePort)
		dstIface, dstFound := findInterface(dstResult, l.TargetPort)
		switch {
		case !srcFound && !dstFound:
			l.ValidationStatus = "not_found"
			l.ValidationDetail = "neither \"" + l.SourcePort + "\" nor \"" + l.TargetPort + "\" matched a real interface"
		case !srcFound:
			l.ValidationStatus = "not_found"
			l.ValidationDetail = "\"" + l.SourcePort + "\" did not match a real interface on the source device"
		case !dstFound:
			l.ValidationStatus = "not_found"
			l.ValidationDetail = "\"" + l.TargetPort + "\" did not match a real interface on the target device"
		case !srcIface.OperUp || !srcIface.AdminUp || !dstIface.OperUp || !dstIface.AdminUp:
			l.ValidationStatus = "down"
			l.ValidationDetail = downDetail(srcIface, dstIface)
		default:
			l.ValidationStatus = "up"
			l.ValidationDetail = ""
		}
		_ = v.Repo.SetLinkValidation(ctx, l.ID, l.ValidationStatus, l.ValidationDetail)
	}
	return links, nil
}

// findInterface matches a stored port string against a discovered
// interface by description first (case-insensitive), falling back to
// index -- mirrors how topology-links' <datalist> populates suggestions
// as `description || index` (feature 24), so a port name saved from that
// suggestion list round-trips correctly here.
func findInterface(result snmp.DiscoveryResult, port string) (snmp.Interface, bool) {
	port = strings.TrimSpace(port)
	for _, iface := range result.Interfaces {
		if strings.EqualFold(strings.TrimSpace(iface.Description), port) {
			return iface, true
		}
	}
	for _, iface := range result.Interfaces {
		if iface.Index == port {
			return iface, true
		}
	}
	return snmp.Interface{}, false
}

func downDetail(a, b snmp.Interface) string {
	switch {
	case !a.AdminUp:
		return "source interface is administratively down"
	case !a.OperUp:
		return "source interface is operationally down"
	case !b.AdminUp:
		return "target interface is administratively down"
	default:
		return "target interface is operationally down"
	}
}
