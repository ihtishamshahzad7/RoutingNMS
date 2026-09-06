"use client";

import { useState } from "react";
import { Bell, Check } from "lucide-react";
import styles from "./glass.module.css";
import { useWorkspaceStore } from "./store";

export default function AlertBell({ groupId }: { groupId: string }) {
  const alerts = useWorkspaceStore((s) => s.alerts.filter((a) => a.groupId === groupId && !a.acknowledged));
  const acknowledgeAlert = useWorkspaceStore((s) => s.acknowledgeAlert);
  const [open, setOpen] = useState(false);

  return (
    <div style={{ position: "relative" }}>
      <button className={styles.bellButton} onClick={() => setOpen((v) => !v)} aria-label="Alerts">
        <Bell size={18} />
        {alerts.length > 0 && <span className={styles.bellBadge}>{alerts.length > 99 ? "99+" : alerts.length}</span>}
      </button>
      {open && (
        <div
          className={styles.glassPanel}
          style={{ position: "absolute", top: 48, right: 0, width: 320, maxHeight: 400, overflowY: "auto", padding: 12, zIndex: 10 }}
        >
          <div className={styles.sectionTitle} style={{ marginTop: 0 }}>Active alerts</div>
          {alerts.length === 0 && <div style={{ fontSize: 12.5, color: "#94a3b8" }}>No active alerts.</div>}
          {alerts.map((a) => (
            <div
              key={a.id}
              className={`${styles.alertRow} ${
                a.severity === "critical" ? styles.alertCritical : a.severity === "warning" ? styles.alertWarning : styles.alertInfo
              }`}
            >
              <div style={{ flex: 1 }}>
                <div>{a.message}</div>
                <div style={{ color: "#64748b", fontSize: 11, marginTop: 2 }}>
                  {new Date(a.firedAt).toLocaleTimeString()}
                </div>
              </div>
              <button
                onClick={() => acknowledgeAlert(a.id)}
                title="Acknowledge"
                style={{ background: "none", border: "none", color: "#94a3b8", cursor: "pointer" }}
              >
                <Check size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
