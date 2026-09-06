"use client";

import { useEffect, useMemo, useState } from "react";
import { LineChart, Line, ResponsiveContainer, YAxis, Tooltip } from "recharts";
import { X, Zap, Radar } from "lucide-react";
import styles from "./glass.module.css";
import { useWorkspaceStore } from "./store";
import { generatePowerMetrics } from "./store";
import { generateUptimeTimeline } from "./mockMetrics";
import { apiFetch } from "../../../lib/api";

// Single-tenant placeholder, same convention used by every other (noc) page.
const ORG = "tenant-1";

type RealDevice = { id: string; name: string };

function MiniChart({ data, color }: { data: { t: number; value: number }[]; color: string }) {
  return (
    <ResponsiveContainer width="100%" height={54}>
      <LineChart data={data}>
        <YAxis hide domain={["dataMin - 2", "dataMax + 2"]} />
        <Tooltip
          contentStyle={{ background: "#0f0f18", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 8, fontSize: 11 }}
          labelFormatter={(t) => new Date(t as number).toLocaleTimeString()}
        />
        <Line type="monotone" dataKey="value" stroke={color} strokeWidth={2} dot={false} isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

export default function MonitoringPanel({ deviceId, onClose }: { deviceId: string; onClose: () => void }) {
  const device = useWorkspaceStore((s) => s.devices.find((d) => d.id === deviceId));
  const metrics = useWorkspaceStore((s) => s.metricsByDevice[deviceId]);
  const alerts = useWorkspaceStore((s) => s.alerts.filter((a) => a.deviceId === deviceId));
  const updateDevice = useWorkspaceStore((s) => s.updateDevice);
  const runDiscovery = useWorkspaceStore((s) => s.runDiscovery);
  const discovering = useWorkspaceStore((s) => s.discovering);

  const [realDevices, setRealDevices] = useState<RealDevice[]>([]);
  useEffect(() => {
    apiFetch<RealDevice[]>(`/devices?organizationId=${ORG}`)
      .then(setRealDevices)
      .catch(() => {}); // real-device linking is optional -- fine if this list can't load
  }, []);

  // Uptime timeline and power metrics are cheap to regenerate per open and
  // don't need to live in the shared store (nothing else reads them).
  const uptime = useMemo(() => generateUptimeTimeline(), [deviceId]);
  const power = useMemo(() => generatePowerMetrics(Math.random() > 0.15), [deviceId]);

  if (!device || !metrics) return null;

  return (
    <div className={styles.monitorPanel}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <div style={{ fontWeight: 600 }}>{device.name}</div>
          <div style={{ fontSize: 12, color: "#94a3b8" }}>{device.kind} &middot; {device.address || "no address set"}</div>
        </div>
        <button onClick={onClose} style={{ background: "none", border: "none", color: "#94a3b8", cursor: "pointer" }}>
          <X size={18} />
        </button>
      </div>

      <div className={styles.sectionTitle}>Real device link</div>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <select
          value={device.linkedDeviceId || ""}
          onChange={(e) => updateDevice(device.id, { linkedDeviceId: e.target.value || undefined })}
          style={{
            flex: 1,
            background: "#0f0f18",
            color: "#e2e8f0",
            border: "1px solid rgba(255,255,255,0.1)",
            borderRadius: 6,
            padding: "4px 8px",
            fontSize: 12.5,
          }}
        >
          <option value="">Not linked (mock monitoring)</option>
          {realDevices.map((d) => (
            <option key={d.id} value={d.id}>{d.name}</option>
          ))}
        </select>
        <button
          onClick={() => runDiscovery(device.groupId, device.id)}
          disabled={!device.linkedDeviceId || discovering}
          title={device.linkedDeviceId ? "Walk this device's real LLDP neighbors" : "Link a real device first"}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 4,
            background: "rgba(34,211,238,0.12)",
            color: device.linkedDeviceId ? "#22d3ee" : "#475569",
            border: "1px solid rgba(34,211,238,0.25)",
            borderRadius: 6,
            padding: "4px 10px",
            fontSize: 12,
            cursor: device.linkedDeviceId && !discovering ? "pointer" : "not-allowed",
            whiteSpace: "nowrap",
          }}
        >
          <Radar size={13} /> {discovering ? "Discovering…" : "Discover"}
        </button>
      </div>

      <div className={styles.sectionTitle}>Bandwidth (Mbps)</div>
      <div style={{ display: "flex", gap: 8 }}>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 11, color: "#64748b" }}>In</div>
          <MiniChart data={metrics.bandwidthIn} color="#22d3ee" />
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 11, color: "#64748b" }}>Out</div>
          <MiniChart data={metrics.bandwidthOut} color="#34d399" />
        </div>
      </div>

      <div className={styles.sectionTitle}>Latency (ms)</div>
      <MiniChart data={metrics.latency} color="#fbbf24" />

      <div className={styles.sectionTitle}>CPU / Memory (%)</div>
      <div style={{ display: "flex", gap: 8 }}>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 11, color: "#64748b" }}>CPU</div>
          <MiniChart data={metrics.cpu} color="#f87171" />
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 11, color: "#64748b" }}>Memory</div>
          <MiniChart data={metrics.memory} color="#a78bfa" />
        </div>
      </div>

      <div className={styles.sectionTitle}>Uptime (last 7 days)</div>
      <div className={styles.uptimeBar} title="Green = up, red = down">
        {uptime.map((seg, i) => (
          <div
            key={i}
            className={seg.status === "up" ? styles.uptimeSegUp : styles.uptimeSegDown}
            style={{ flexGrow: seg.end - seg.start }}
            title={
              seg.status === "up"
                ? `Up ${new Date(seg.start).toLocaleString()} → ${new Date(seg.end).toLocaleString()}`
                : `Down: ${seg.reason} (${new Date(seg.start).toLocaleString()})`
            }
          />
        ))}
      </div>

      <div className={styles.sectionTitle}>Power</div>
      <div style={{ display: "flex", gap: 16, fontSize: 12.5, marginBottom: 6 }}>
        <div><Zap size={12} style={{ display: "inline", marginRight: 4 }} />{power.voltage.toFixed(1)} V</div>
        <div>{power.current.toFixed(2)} A</div>
      </div>
      {power.psu.map((p) => (
        <div key={p.id} className={styles.psuRow}>
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: 999,
              background: p.healthy ? "#34d399" : "#f87171",
              boxShadow: p.healthy ? "0 0 4px #34d399" : "0 0 4px #f87171",
            }}
          />
          {p.label} &mdash; {p.healthy ? "healthy" : "FAULT"}
        </div>
      ))}

      <div className={styles.sectionTitle}>Active alerts ({alerts.filter((a) => !a.acknowledged).length})</div>
      {alerts.length === 0 && <div style={{ fontSize: 12, color: "#64748b" }}>No alerts for this device.</div>}
      {alerts.map((a) => (
        <div
          key={a.id}
          className={`${styles.alertRow} ${
            a.severity === "critical" ? styles.alertCritical : a.severity === "warning" ? styles.alertWarning : styles.alertInfo
          }`}
          style={{ opacity: a.acknowledged ? 0.45 : 1 }}
        >
          {a.message}
        </div>
      ))}
    </div>
  );
}
