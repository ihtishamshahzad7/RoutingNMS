"use client";

// 40px NOC top bar: logo + system status on the left, open-alerts badge +
// pulse on the right. The alerts badge is fed from the real backend endpoint
// GET /api/v1/alerts/active (via the Zustand alert store), so every surface
// shows one consistent live count.
import { useEffect, useState } from "react";
import { useAlertStore, type ActiveAlert } from "../lib/stores/alerts";
import { apiFetch } from "../lib/api";
import { AlertBadge } from "./ui/primitives";

export function TopBar() {
  const alerts = useAlertStore((s) => s.alerts);
  const setAlerts = useAlertStore((s) => s.setAlerts);
  const [now, setNow] = useState<Date>(() => new Date());

  useEffect(() => {
    const t = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(t);
  }, []);

  useEffect(() => {
    let active = true;
    const poll = async () => {
      try {
        const data = await apiFetch<ActiveAlert[]>("/alerts/active");
        if (active) setAlerts(data);
      } catch {
        // Backend unreachable / no session: leave the badge as-is rather than
        // showing a misleading 0. The next poll retries.
      }
    };
    poll();
    const t = window.setInterval(poll, 15000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
  }, [setAlerts]);

  const openCount = alerts.filter(
    (a) => a.severity === "critical" || a.severity === "warning"
  ).length;

  const utc = now.toUTCString();

  return (
    <header className="flex h-10 shrink-0 items-center justify-between border-b border-border bg-bg-surface px-4">
      <div className="flex items-center gap-3">
        <span className="text-sm font-bold tracking-tight text-text-primary">
          Routing<span className="text-status-up">NMS</span>
        </span>
        <span className="dot dot-up" />
        <span className="text-xs text-text-muted">All systems nominal</span>
        <span className="mono text-text-ghost">{utc}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className="label">Open alerts</span>
        <AlertBadge count={openCount} />
        <span className="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-status-up animate-pulse" />
      </div>
    </header>
  );
}