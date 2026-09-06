package workspacetopology

// Real SNMP/LLDP neighbor discovery for the Workspace Topology Builder's
// "Discover" action -- closes the follow-up flagged since feature 26
// (originally a pure client-side mock, see the frontend's discovery.ts).
//
// This only works from a canvas device that's linked to a real, monitored
// RoutingNMS device (CanvasDevice.linkedDeviceId, modeled since feature 26
// but unused until now): that's the one place we have real SNMP
// credentials to walk from. A canvas device with no linked real device has
// nothing to discover from and falls back to the frontend's existing mock
// (frontend/app/(noc)/workspace-topology/store.ts's runDiscovery already
// has that fallback wired in).
//
// Reuses internal/topology's SNMPNeighborDiscovery (the same LLDP-MIB
// walker the scheduled topology-links discovery engine already uses) rather
// than re-implementing an LLDP walk -- see internal/topology/lldp.go /
// snmp_lldp.go (naming carried over from that package; it does the real
// walk despite the "snmp_lldp.go" filename suggesting otherwise).

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/devices"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/topology"
)

// DiscoveryService walks a linked device's LLDP neighbors and persists any
// newly-discovered neighbors as canvas devices/links in the same workspace
// group, matching each neighbor back to a known RoutingNMS device by name
// when possible (so the new canvas node inherits a real linkedDeviceId and
// address, same as if the user had linked it by hand).
type DiscoveryService struct {
	Repo    Repository
	Devices devices.Repository
}

// DiscoverFromSeed runs one discovery pass from seedDeviceID (a canvas
// device id) and returns the newly-created canvas devices/links. Idempotent
// across repeated calls: a neighbor already represented by a canvas device
// in this group (matched by linkedDeviceId) is not duplicated, though a
// missing seed<->neighbor link is still added.
func (s DiscoveryService) DiscoverFromSeed(ctx context.Context, groupID int64, seedDeviceID string) ([]Device, []Link, error) {
	if s.Repo.DB == nil {
		return nil, nil, fmt.Errorf("workspacetopology repository is not initialized")
	}
	seed, err := s.Repo.DeviceByID(ctx, seedDeviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("load seed device: %w", err)
	}
	if seed.GroupID != groupID {
		return nil, nil, fmt.Errorf("seed device does not belong to this group")
	}
	if seed.LinkedDeviceID == nil || strings.TrimSpace(*seed.LinkedDeviceID) == "" {
		return nil, nil, fmt.Errorf("this canvas device is not linked to a monitored device -- link it to a real device first to run real discovery")
	}

	input, err := s.Devices.DiscoveryTarget(ctx, *seed.LinkedDeviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("load SNMP target: %w", err)
	}
	target := snmp.Target{
		ID:          *seed.LinkedDeviceID,
		Address:     input.Address,
		Port:        input.SNMPPort,
		Credentials: input.SNMP,
		Timeout:     input.Timeout,
		Retries:     1,
	}
	discovery := topology.SNMPNeighborDiscovery{
		Collector: snmp.Collector{},
		Resolve:   func(topology.Node) (snmp.Target, error) { return target, nil },
	}
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	neighbors, err := discovery.Discover(probeCtx, topology.Node{ID: *seed.LinkedDeviceID, Address: input.Address})
	if err != nil {
		return nil, nil, fmt.Errorf("LLDP discovery: %w", err)
	}

	// Match each neighbor's remote system name back to a known enabled
	// device, so a recognized neighbor inherits a real linkedDeviceId and
	// address -- same resolution approach internal/topology's own
	// scheduled discovery engine uses for topology_links (see engine.go's
	// keyName/byName matching).
	known, err := s.Devices.ListAllEnabled(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load known devices: %w", err)
	}
	byName := map[string]devices.Record{}
	for _, d := range known {
		byName[keyName(d.Name)] = d
	}

	existingDevices, err := s.Repo.DevicesOf(ctx, groupID)
	if err != nil {
		return nil, nil, fmt.Errorf("load existing group devices: %w", err)
	}
	linkedAlready := map[string]string{} // real device id -> existing canvas device id
	for _, d := range existingDevices {
		if d.LinkedDeviceID != nil {
			linkedAlready[*d.LinkedDeviceID] = d.ID
		}
	}
	existingLinks, err := s.Repo.LinksOf(ctx, groupID)
	if err != nil {
		return nil, nil, fmt.Errorf("load existing group links: %w", err)
	}
	linkExists := func(a, b string) bool {
		for _, l := range existingLinks {
			if (l.SourceID == a && l.TargetID == b) || (l.SourceID == b && l.TargetID == a) {
				return true
			}
		}
		return false
	}

	var newDevices []Device
	var newLinks []Link
	i := 0
	for _, n := range neighbors {
		name := strings.TrimSpace(n.RemoteID)
		if name == "" {
			continue
		}
		matched, ok := byName[keyName(name)]

		var canvasID string
		if ok {
			if existingID, already := linkedAlready[matched.ID]; already {
				// Already represented in this group -- just make sure the
				// seed<->neighbor link exists, don't duplicate the device.
				canvasID = existingID
			}
		}
		if canvasID == "" {
			i++
			angle := float64(i) * (2 * math.Pi / 8)
			dev := Device{
				ID:      fmt.Sprintf("discovered-%d-%s-%d-%d", groupID, safeIDPart(seedDeviceID), i, time.Now().UnixNano()%1_000_000),
				GroupID: groupID,
				Name:    name,
				Kind:    "switch", // LLDP-MIB doesn't expose a device kind; default matches the mock discovery's convention
				PosX:    seed.PosX + 220*math.Cos(angle),
				PosY:    seed.PosY + 220*math.Sin(angle),
			}
			if ok {
				id := matched.ID
				dev.LinkedDeviceID = &id
				dev.Address = matched.Address
			}
			created, err := s.Repo.CreateDevice(ctx, dev)
			if err != nil {
				continue // one bad neighbor shouldn't abort the whole discovery pass
			}
			newDevices = append(newDevices, created)
			canvasID = created.ID
			if ok {
				linkedAlready[matched.ID] = canvasID
			}
		}

		if linkExists(seedDeviceID, canvasID) {
			continue
		}
		link := Link{
			ID:         fmt.Sprintf("discovered-link-%d-%s-%d-%d", groupID, safeIDPart(seedDeviceID), i, time.Now().UnixNano()%1_000_000),
			GroupID:    groupID,
			SourceID:   seedDeviceID,
			TargetID:   canvasID,
			Discovered: true,
		}
		createdLink, err := s.Repo.CreateLink(ctx, link)
		if err != nil {
			continue
		}
		newLinks = append(newLinks, createdLink)
		existingLinks = append(existingLinks, createdLink)
	}

	return newDevices, newLinks, nil
}

func keyName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// safeIDPart strips the "dev-" prefix nextId() puts on client-generated
// canvas device ids, purely for readability of the generated discovered-*
// ids -- not load-bearing.
func safeIDPart(id string) string { return strings.TrimPrefix(id, "dev-") }
