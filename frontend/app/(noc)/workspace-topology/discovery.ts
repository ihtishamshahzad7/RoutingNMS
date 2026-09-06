// Auto-discovery engine, mocked for this sprint.
//
// The real thing this stands in for: RoutingNMS already has a working SNMP
// walker (backend/internal/snmp.Collector.Discover, IF-MIB ifTable) exposed
// at POST /api/v1/devices/{id}/discover -- see the topology-links feature,
// which already calls it for live interface suggestions. It does NOT yet
// walk lldpRemTable or cdpCacheTable (LLDP/CDP neighbor discovery), which is
// what this feature actually needs to auto-populate a canvas with
// neighboring devices and the links between them -- that is new backend
// work, out of scope for this mocked sprint per the request ("Backend
// (mocked for now, but designed for integration)").
//
// discoverFromSeed() is written as the seam: swap its body for a real fetch
// to a future `POST /api/v1/discovery/lldp-cdp?seed=<deviceId>` endpoint and
// nothing else in this feature needs to change, since callers only depend
// on the DiscoveryResult shape below.

import type { CanvasDevice, CanvasLink, DeviceKind } from "./types";

export type DiscoveryResult = {
  devices: CanvasDevice[];
  links: CanvasLink[];
};

const MOCK_KINDS: DeviceKind[] = ["router", "switch", "firewall", "server"];

function randomAddress(): string {
  return `10.${Math.floor(Math.random() * 254) + 1}.${Math.floor(Math.random() * 254) + 1}.${Math.floor(Math.random() * 254) + 1}`;
}

/**
 * Simulates an SNMP/LLDP/CDP neighbor walk starting from one seed device.
 * Returns a small, plausible neighbor set with links back to the seed --
 * enough to demonstrate "click Discover, canvas populates" end-to-end.
 * Resolves after a short delay to mimic real network round-trips so the
 * UI's loading state is exercised too.
 */
export async function discoverFromSeed(
  groupId: string,
  seed: CanvasDevice
): Promise<DiscoveryResult> {
  await new Promise((r) => setTimeout(r, 900 + Math.random() * 600));

  const neighborCount = 2 + Math.floor(Math.random() * 3); // 2-4 neighbors
  const devices: CanvasDevice[] = [];
  const links: CanvasLink[] = [];
  const angleStep = (Math.PI * 2) / neighborCount;

  for (let i = 0; i < neighborCount; i++) {
    const kind = MOCK_KINDS[Math.floor(Math.random() * MOCK_KINDS.length)];
    const angle = angleStep * i;
    const radius = 220;
    const device: CanvasDevice = {
      id: `disc-${seed.id}-${i}-${Date.now()}`,
      groupId,
      name: `${kind}-${Math.floor(Math.random() * 900 + 100)}`,
      kind,
      address: randomAddress(),
      x: seed.x + Math.cos(angle) * radius,
      y: seed.y + Math.sin(angle) * radius,
    };
    devices.push(device);
    links.push({
      id: `disc-link-${seed.id}-${device.id}`,
      groupId,
      sourceId: seed.id,
      targetId: device.id,
      sourcePort: `Gi0/${i + 1}`,
      targetPort: "Gi0/1",
      discovered: true,
    });
  }

  return { devices, links };
}
