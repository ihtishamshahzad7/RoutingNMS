"use client";

import { useEffect, useState } from "react";
import { Plus, Pencil, Trash2, Router, Server, ShieldAlert, Network as NetworkIcon, Radar, LayoutGrid, ShieldCheck } from "lucide-react";
import styles from "./glass.module.css";
import { useWorkspaceStore } from "./store";
import { useLiveStream } from "./useLiveStream";
import Canvas from "./Canvas";
import MonitoringPanel from "./MonitoringPanel";
import AlertBell from "./AlertBell";
import type { DeviceKind } from "./types";

const PALETTE: { kind: DeviceKind; label: string; icon: React.ComponentType<{ size?: number }> }[] = [
  { kind: "router", label: "Router", icon: Router },
  { kind: "switch", label: "Switch", icon: NetworkIcon },
  { kind: "firewall", label: "Firewall", icon: ShieldAlert },
  { kind: "server", label: "Server", icon: Server },
];

/**
 * Workspace Topology Builder -- a self-contained sub-app under
 * /workspace-topology. Everything here is additive: it doesn't touch any
 * existing route, the existing /topology (auto-generated LLDP graph) or
 * /topology-links (manual port-level link mapping backed by real
 * metric_samples) pages, or the app's shared design system. State lives
 * entirely client-side in the Zustand store (./store.ts) for this sprint --
 * per the request, the backend (discovery + live metrics) is mocked; see
 * ./discovery.ts and ./mockMetrics.ts for the integration seams a future
 * sprint would replace with real RoutingNMS API calls.
 */
export default function WorkspaceTopologyPage() {
  const groups = useWorkspaceStore((s) => s.groups);
  const activeGroupId = useWorkspaceStore((s) => s.activeGroupId);
  const createGroup = useWorkspaceStore((s) => s.createGroup);
  const renameGroup = useWorkspaceStore((s) => s.renameGroup);
  const deleteGroup = useWorkspaceStore((s) => s.deleteGroup);
  const setActiveGroup = useWorkspaceStore((s) => s.setActiveGroup);
  const addDevice = useWorkspaceStore((s) => s.addDevice);
  const devices = useWorkspaceStore((s) => s.devices);
  const selectedDeviceId = useWorkspaceStore((s) => s.selectedDeviceId);
  const selectDevice = useWorkspaceStore((s) => s.selectDevice);
  const runDiscovery = useWorkspaceStore((s) => s.runDiscovery);
  const discovering = useWorkspaceStore((s) => s.discovering);
  const autoLayout = useWorkspaceStore((s) => s.autoLayout);
  const validateLinks = useWorkspaceStore((s) => s.validateLinks);
  const validating = useWorkspaceStore((s) => s.validating);
  const links = useWorkspaceStore((s) => s.links);
  const tickMetrics = useWorkspaceStore((s) => s.tickMetrics);
  const loadGroups = useWorkspaceStore((s) => s.loadGroups);
  const liveStreamConnected = useWorkspaceStore((s) => s.liveStreamConnected);

  const [newGroupName, setNewGroupName] = useState("");

  // Load persisted groups (and, via setActiveGroup once one becomes active,
  // that group's devices/links) once on mount.
  useEffect(() => {
    loadGroups();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Real-time monitoring tick: every 5s, advance every visible device's
  // mocked metrics and evaluate alert thresholds. Swappable for a
  // WebSocket/SSE subscription later without changing any consumer, since
  // components only read from the store.
  useEffect(() => {
    const id = setInterval(tickMetrics, 5000);
    return () => clearInterval(id);
  }, [tickMetrics]);

  const activeGroup = groups.find((g) => g.id === activeGroupId);
  const groupDevices = devices.filter((d) => d.groupId === activeGroupId);
  const groupLinks = links.filter((l) => l.groupId === activeGroupId);

  // Real-time push for linked devices' up/latency state (see
  // useLiveStream.ts + backend workspacetopology.LiveHub); a no-op when no
  // group is active. tickMetrics (below) keeps advancing the mock
  // bandwidth/CPU/memory/fallback-latency series independently.
  useLiveStream(activeGroup?.id ?? null);

  const handleCreateGroup = () => {
    const name = newGroupName.trim();
    if (!name) return;
    createGroup(name);
    setNewGroupName("");
  };

  const handleAddDevice = (kind: DeviceKind) => {
    if (!activeGroupId) return;
    const cx = 300 + Math.random() * 200 - 100;
    const cy = 220 + Math.random() * 160 - 80;
    addDevice(activeGroupId, kind, cx, cy);
  };

  const handleDiscover = () => {
    if (!activeGroupId || groupDevices.length === 0) return;
    // Seed from the first device in the group -- in a real integration the
    // user would pick the seed explicitly (e.g. right-click "Discover from
    // here"), left as a follow-up since this sprint's ask was end-to-end
    // plumbing, not seed-selection UX polish.
    runDiscovery(activeGroupId, groupDevices[0].id);
  };

  return (
    <div className={styles.page}>
      <aside className={styles.sidebar}>
        <div className={styles.sidebarTitle}>Workspace groups</div>
        <div style={{ display: "flex", gap: 6 }}>
          <input
            value={newGroupName}
            onChange={(e) => setNewGroupName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleCreateGroup()}
            placeholder="e.g. Data Center A"
            style={{
              flex: 1,
              background: "rgba(255,255,255,0.04)",
              border: "1px solid rgba(255,255,255,0.08)",
              borderRadius: 8,
              padding: "6px 8px",
              color: "#e5e7eb",
              fontSize: 12.5,
            }}
          />
          <button className={styles.paletteButtonPrimary} onClick={handleCreateGroup} aria-label="Create group">
            <Plus size={14} />
          </button>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 4, marginTop: 4 }}>
          {groups.length === 0 && (
            <div style={{ fontSize: 12, color: "#64748b", padding: "6px 4px" }}>
              No groups yet. Create one above (e.g. &ldquo;Data Center A&rdquo;, &ldquo;Branch Office&rdquo;, &ldquo;ISP Edge&rdquo;).
            </div>
          )}
          {groups.map((g) => (
            <div
              key={g.id}
              className={`${styles.groupRow} ${g.id === activeGroupId ? styles.groupRowActive : ""}`}
              onClick={() => setActiveGroup(g.id)}
            >
              <span style={{ fontSize: 13, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{g.name}</span>
              <span style={{ display: "flex", gap: 4 }}>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    const name = prompt("Rename group", g.name);
                    if (name && name.trim()) renameGroup(g.id, name.trim());
                  }}
                  style={{ background: "none", border: "none", color: "#94a3b8", cursor: "pointer" }}
                  aria-label="Rename group"
                >
                  <Pencil size={12} />
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    if (confirm(`Delete group "${g.name}"? This removes its devices and links too.`)) deleteGroup(g.id);
                  }}
                  style={{ background: "none", border: "none", color: "#94a3b8", cursor: "pointer" }}
                  aria-label="Delete group"
                >
                  <Trash2 size={12} />
                </button>
              </span>
            </div>
          ))}
        </div>
      </aside>

      <main className={styles.canvasArea}>
        {!activeGroup ? (
          <div className={styles.canvasSurface} style={{ display: "flex", alignItems: "center", justifyContent: "center", color: "#64748b", fontSize: 14 }}>
            Select or create a group to open its workspace canvas.
          </div>
        ) : (
          <Canvas groupId={activeGroup.id} />
        )}

        {activeGroup && (
          <>
            <div className={styles.toolbar}>
              {PALETTE.map((p) => {
                const Icon = p.icon;
                return (
                  <button key={p.kind} className={styles.paletteButton} onClick={() => handleAddDevice(p.kind)}>
                    <Icon size={14} />
                    {p.label}
                  </button>
                );
              })}
              <button
                className={styles.paletteButtonPrimary}
                data-busy={discovering}
                onClick={handleDiscover}
                disabled={groupDevices.length === 0}
                title={groupDevices.length === 0 ? "Add a seed device first" : "Discover neighbors via SNMP/LLDP/CDP"}
              >
                <Radar size={14} />
                {discovering ? "Discovering…" : "Discover"}
              </button>
              <button
                className={styles.paletteButton}
                onClick={() => activeGroup && autoLayout(activeGroup.id)}
                disabled={groupDevices.length === 0}
                title={groupDevices.length === 0 ? "Add devices first" : "Auto-arrange devices with a force-directed layout"}
              >
                <LayoutGrid size={14} />
                Auto Layout
              </button>
              <button
                className={styles.paletteButton}
                data-busy={validating}
                onClick={() => activeGroup && validateLinks(activeGroup.id)}
                disabled={groupLinks.length === 0 || validating}
                title={
                  groupLinks.length === 0
                    ? "Add a link first"
                    : "Check every link's ports against real SNMP interface state"
                }
              >
                <ShieldCheck size={14} />
                {validating ? "Validating…" : "Validate Links"}
              </button>
              {groupDevices.some((d) => d.linkedDeviceId) && (
                <span
                  title={
                    liveStreamConnected
                      ? "Receiving real up/latency updates for linked devices"
                      : "Connecting to the live updates stream…"
                  }
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 6,
                    marginLeft: "auto",
                    fontSize: 11.5,
                    color: liveStreamConnected ? "#34d399" : "#94a3b8",
                    padding: "4px 10px",
                  }}
                >
                  <span
                    className={liveStreamConnected ? styles.livePulse : undefined}
                    style={{
                      width: 7,
                      height: 7,
                      borderRadius: 999,
                      background: liveStreamConnected ? "#34d399" : "#475569",
                      boxShadow: liveStreamConnected ? "0 0 6px #34d399" : "none",
                    }}
                  />
                  {liveStreamConnected ? "Live" : "Connecting…"}
                </span>
              )}
            </div>
            <div style={{ position: "absolute", top: 16, right: activeGroup && selectedDeviceId ? 396 : 16, zIndex: 6, transition: "right 0.2s ease" }}>
              <AlertBell groupId={activeGroup.id} />
            </div>
          </>
        )}

        {activeGroup && selectedDeviceId && groupDevices.some((d) => d.id === selectedDeviceId) && (
          <MonitoringPanel deviceId={selectedDeviceId} onClose={() => selectDevice(null)} />
        )}
      </main>
    </div>
  );
}
