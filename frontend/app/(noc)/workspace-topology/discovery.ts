// Auto-discovery engine.
//
// Real discovery landed: when the seed canvas device is linked to a real,
// monitored RoutingNMS device (CanvasDevice.linkedDeviceId), discoverReal()
// below calls POST /api/v1/workspace-topology/groups/{groupId}/discover,
// which walks that device's real LLDP-MIB neighbors (backend/internal/
// topology's existing SNMPNeighborDiscovery, the same walker the scheduled
// topology-links discovery engine uses) and persists any newly-found
// neighbors as canvas devices/links server-side.
//
// discoverFromSeed() (the mock below) is kept as the fallback for a canvas
// device with no linkedDeviceId -- there's nothing real to walk from in
// that case, so store.ts's runDiscovery still falls back to this to keep
// the "click Discover, canvas populates" demo usable for an unlinked/
// exploratory workspace.

import type { CanvasDevice, CanvasLink, DeviceKind } from "./types";
import { apiFetch } from "../../../lib/api";

export type DiscoveryResult = {
  devices: CanvasDevice[];
  links: CanvasLink[];
};

/**
 * Runs real SNMP/LLDP neighbor discovery from a seed canvas device that's
 * linked to a monitored RoutingNMS device. Throws if the seed isn't linked,
 * the backend can't reach it over SNMP, or the request otherwise fails --
 * callers should catch and fall back to discoverFromSeed() (the mock).
 */
export async function discoverReal(groupId: string, seed: CanvasDevice): Promise<DiscoveryResult> {
  if (!seed.linkedDeviceId) {
    throw new Error("seed device is not linked to a monitored device");
  }
  return apiFetch<DiscoveryResult>(`/workspace-topology/groups/${groupId}/discover`, {
    method: "POST",
    body: JSON.stringify({ seedDeviceId: seed.id }),
  });
}

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
