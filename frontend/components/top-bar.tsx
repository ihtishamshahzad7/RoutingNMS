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
    <header className="flex h-10 shrink-0 items-center justify-between border-b border-[#DCE1E8] bg-white px-4">
      <div className="flex items-center gap-3">
        <span className="h-1.5 w-1.5 rounded-full bg-[#1E8E5A]" />
        <span className="text-xs text-[#5C6B7A]">All systems nominal</span>
        <span className="font-mono text-xs text-[#8A96A3]">{utc}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-[11px] font-medium uppercase tracking-wide text-[#5C6B7A]">Open alerts</span>
        <AlertBadge count={openCount} />
        <span className="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-[#1E8E5A] animate-pulse" />
      </div>
    </header>
  );
}