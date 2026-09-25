"use client";

import { useCallback, useRef, useState } from "react";
import { Router, Server, ShieldAlert, Network, Zap } from "lucide-react";
import styles from "./glass.module.css";
import { useWorkspaceStore } from "./store";
import type { CanvasDevice, DeviceKind } from "./types";

const KIND_ICON: Record<DeviceKind, React.ComponentType<{ size?: number }>> = {
  router: Router,
  switch: Network,
  firewall: ShieldAlert,
  server: Server,
  power: Zap,
};

/**
 * EVE-NG style workspace canvas: pan (drag empty space) + zoom (wheel),
 * drag-and-drop devices, and click-to-connect link drawing. Built on plain
 * SVG + pointer events rather than React Flow/Konva/Three.js -- d3 is
 * already a dependency here (it renders the existing auto-generated
 * /topology LLDP graph) but this canvas doesn't even need d3's force
 * simulation since device positions are user-placed, not auto-laid-out, so
 * plain SVG keeps this feature dependency-free. Swappable for React Flow
 * later without changing the store if a heavier canvas is ever justified.
 */
export default function Canvas({ groupId }: { groupId: string }) {
  const devices = useWorkspaceStore((s) => s.devices.filter((d) => d.groupId === groupId));
  const links = useWorkspaceStore((s) => s.links.filter((l) => l.groupId === groupId));
  const selectedDeviceId = useWorkspaceStore((s) => s.selectedDeviceId);
  const moveDevice = useWorkspaceStore((s) => s.moveDevice);
  const addLink = useWorkspaceStore((s) => s.addLink);
  const selectDevice = useWorkspaceStore((s) => s.selectDevice);

  const surfaceRef = useRef<HTMLDivElement>(null);
  const [viewport, setViewport] = useState({ x: 0, y: 0, scale: 1 });
  const [linkFrom, setLinkFrom] = useState<string | null>(null);
  const [dragging, setDragging] = useState<{ id: string; dx: number; dy: number } | null>(null);
  const [panning, setPanning] = useState<{ startX: number; startY: number; ox: number; oy: number } | null>(null);

  const toWorld = useCallback(
    (clientX: number, clientY: number) => {
      const rect = surfaceRef.current?.getBoundingClientRect();
      if (!rect) return { x: 0, y: 0 };
      return {
        x: (clientX - rect.left - viewport.x) / viewport.scale,
        y: (clientY - rect.top - viewport.y) / viewport.scale,
      };
    },
    [viewport]
  );

  const onDeviceMouseDown = (e: React.MouseEvent, device: CanvasDevice) => {
    e.stopPropagation();
    const world = toWorld(e.clientX, e.clientY);
    setDragging({ id: device.id, dx: world.x - device.x, dy: world.y - device.y });
    selectDevice(device.id);
  };

  const onDeviceClickForLink = (e: React.MouseEvent, device: CanvasDevice) => {
    if (!e.shiftKey) return;
    e.stopPropagation();
    if (!linkFrom) {
      setLinkFrom(device.id);
    } else if (linkFrom !== device.id) {
      addLink(groupId, linkFrom, device.id);
      setLinkFrom(null);
    }
  };

  const onSurfaceMouseMove = (e: React.MouseEvent) => {
    if (dragging) {
      const world = toWorld(e.clientX, e.clientY);
      moveDevice(dragging.id, world.x - dragging.dx, world.y - dragging.dy);
    } else if (panning) {
      setViewport((v) => ({ ...v, x: panning.ox + (e.clientX - panning.startX), y: panning.oy + (e.clientY - panning.startY) }));
    }
  };

  const onSurfaceMouseUp = () => {
    setDragging(null);
    setPanning(null);
  };

  const onSurfaceMouseDown = (e: React.MouseEvent) => {
    if (e.target !== e.currentTarget) return;
    setPanning({ startX: e.clientX, startY: e.clientY, ox: viewport.x, oy: viewport.y });
    selectDevice(null);
    setLinkFrom(null);
  };

  const onWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const delta = e.deltaY > 0 ? 0.92 : 1.08;
    setViewport((v) => ({ ...v, scale: Math.min(2.5, Math.max(0.4, v.scale * delta)) }));
  };

  const findDevice = (id: string) => devices.find((d) => d.id === id);

  // Deterministic per-device online/offline glyph so it doesn't flicker on
  // every metrics-driven re-render (mocked; a real integration would read
  // the device's actual last-poll status instead of hashing its id).
  const isOnline = (id: string) => {
    let h = 0;
    for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
    return h % 100 > 12;
  };

  return (
    <div
      ref={surfaceRef}
      className={styles.canvasSurface}
      onMouseMove={onSurfaceMouseMove}
      onMouseUp={onSurfaceMouseUp}
      onMouseLeave={onSurfaceMouseUp}
      onMouseDown={onSurfaceMouseDown}
      onWheel={onWheel}
      style={{ cursor: panning ? "grabbing" : "default" }}
    >
      <svg
        width="100%"
        height="100%"
        style={{ position: "absolute", inset: 0, pointerEvents: "none" }}
      >
        <g transform={`translate(${viewport.x} ${viewport.y}) scale(${viewport.scale})`}>
          {links.map((link) => {
            const a = findDevice(link.sourceId);
            const b = findDevice(link.targetId);
            if (!a || !b) return null;
            // "unverified" (the default, or absent on an optimistic
            // just-created link) gets no extra class -- keeps the
            // original cyan look rather than implying a check already
            // happened. See glass.module.css for the color meanings.
            const validationClass =
              link.validationStatus === "up"
                ? styles.linkValidUp
                : link.validationStatus === "down"
                ? styles.linkValidDown
                : link.validationStatus === "not_found" || link.validationStatus === "error"
                ? styles.linkValidNotFound
                : "";
            return (
              <line
                key={link.id}
                x1={a.x}
                y1={a.y}
                x2={b.x}
                y2={b.y}
                className={`${styles.linkPulse} ${link.discovered ? styles.linkDiscovered : ""} ${validationClass}`}
                // The parent <svg> disables pointer events (it's a
                // decorative overlay -- the device overlay div below is
                // the interactive layer), so a validated line needs its
                // own pointer-events override for its hover tooltip to
                // actually fire.
                style={link.validationDetail ? { pointerEvents: "stroke", cursor: "default" } : undefined}
              >
                {link.validationDetail && <title>{link.validationDetail}</title>}
              </line>
            );
          })}
        </g>
      </svg>

      <div
        style={{
          position: "absolute",
          inset: 0,
          transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.scale})`,
          transformOrigin: "0 0",
        }}
      >
        {devices.map((device) => {
          const Icon = KIND_ICON[device.kind];
          const selected = device.id === selectedDeviceId;
          return (
            <div
              key={device.id}
              className={styles.deviceNode}
              style={{ left: device.x, top: device.y }}
              onMouseDown={(e) => onDeviceMouseDown(e, device)}
              onClick={(e) => onDeviceClickForLink(e, device)}
              title="Drag to move · Shift+click a second device to link"
            >
              <div className={selected ? styles.deviceGlyphSelected : styles.deviceGlyph}>
                <Icon size={24} />
                <span className={`${styles.statusDot} ${isOnline(device.id) ? styles.statusOnline : styles.statusOffline}`} />
              </div>
              <div className={styles.deviceLabel}>{device.name}</div>
              {linkFrom === device.id && (
                <div style={{ textAlign: "center", fontSize: 10, color: "#22d3ee", marginTop: 2 }}>
                  linking&hellip;
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
