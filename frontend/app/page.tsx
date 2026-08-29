"use client";

import { useEffect, useMemo, useState } from "react";

type RuntimeState = { oltId: string; running: boolean; startedAt?: string; lastPollAt?: string; lastError?: string; pollCount: number };
type Alert = { id: number; oltId: string; ponId: string; onuId: string; code: string; severity: string; message: string; status: string; lastSeen: string };

function formatTime(value?: string) { return value ? new Date(value).toLocaleString() : "—"; }

export default function Home() {
  const [connected, setConnected] = useState(false);
  const [states, setStates] = useState<RuntimeState[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const health = await fetch("/api/v1/health", { cache: "no-store" });
        if (!active) return;
        setConnected(health.ok);
        const runtime = await fetch("/api/v1/olt/runtime", { cache: "no-store" });
        if (runtime.ok) {
          const data = await runtime.json();
          if (active) setStates(Array.isArray(data.olts) ? data.olts : []);
        }
      } catch { if (active) setConnected(false); }
    };
    load();
    const timer = window.setInterval(load, 10000);
    return () => { active = false; window.clearInterval(timer); };
  }, []);

  useEffect(() => {
    let active = true;
    const loadAlerts = async () => {
      const results = await Promise.all(states.filter((s) => s.oltId).map(async (s) => {
        try { const r = await fetch(`/api/v1/olts/${encodeURIComponent(s.oltId)}/alerts?limit=10`, { cache: "no-store" }); return r.ok ? await r.json() : []; }
        catch { return []; }
      }));
      if (active) setAlerts(results.flat().filter((a: Alert) => a.status === "open").slice(0, 20));
    };
    if (states.length) loadAlerts(); else setAlerts([]);
    return () => { active = false; };
  }, [states]);

  const running = states.filter((s) => s.running).length;
  const critical = alerts.filter((a) => a.severity.toLowerCase() === "critical").length;
  const networkHealth = states.length ? Math.round((running / states.length) * 100) : 100;
  const totalPolls = useMemo(() => states.reduce((n, s) => n + s.pollCount, 0), [states]);

  return (
    <main className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800 px-6 py-4"><div className="mx-auto flex max-w-7xl items-center justify-between"><div><div className="text-xl font-bold">RoutingNMS</div><div className="text-xs text-slate-400">Network Operations Center</div></div><div className="flex items-center gap-2 text-xs text-slate-300"><span className={`h-2 w-2 rounded-full ${connected ? "bg-emerald-400" : "bg-amber-400"}`} />{connected ? "Backend connected" : "Backend pending"}</div></div></header>
      <section className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-8"><p className="text-sm font-medium text-cyan-400">LIVE NOC</p><h1 className="mt-1 text-3xl font-bold">Network Overview</h1><p className="mt-2 text-sm text-slate-400">Real-time visibility for routers, switches, OLTs and access infrastructure.</p></div>
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {[
            ["Network Health", `${networkHealth}%`, states.length ? `${running}/${states.length} OLTs running` : "No OLTs registered"],
            ["OLT Pollers", String(running), `${totalPolls} polls completed`],
            ["Critical Alerts", String(critical), critical ? "Immediate attention required" : "No active critical alerts"],
            ["Open Alerts", String(alerts.length), alerts.length ? "Active incidents" : "No active incidents"],
          ].map(([label, value, detail]) => <article key={label} className="rounded-xl border border-slate-800 bg-slate-900 p-5"><div className="text-sm text-slate-400">{label}</div><div className="mt-2 text-3xl font-semibold">{value}</div><div className="mt-2 text-xs text-slate-500">{detail}</div></article>)}
        </div>
        <div className="mt-6 grid gap-6 lg:grid-cols-3">
          <section className="rounded-xl border border-slate-800 bg-slate-900 p-5 lg:col-span-2"><div className="flex justify-between"><h2 className="font-semibold">OLT Runtime</h2><span className="text-xs text-slate-500">Refreshes every 10s</span></div><div className="mt-4 space-y-2">{states.length ? states.map((s) => <div key={s.oltId} className="rounded-lg border border-slate-800 p-4"><div className="flex items-center justify-between"><div className="font-medium">{s.oltId}</div><span className={`rounded-full px-2 py-1 text-xs ${s.running ? "bg-emerald-950 text-emerald-300" : "bg-slate-800 text-slate-400"}`}>{s.running ? "Running" : "Stopped"}</span></div><div className="mt-2 grid gap-2 text-xs text-slate-500 sm:grid-cols-3"><span>Polls: {s.pollCount}</span><span>Last poll: {formatTime(s.lastPollAt)}</span><span>{s.lastError ? `Error: ${s.lastError}` : "Healthy"}</span></div></div>) : <div className="flex min-h-32 items-center justify-center rounded-lg border border-dashed border-slate-800 text-sm text-slate-500">No OLT runtime data available.</div>}</div></section>
          <section className="rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Active Alerts</h2><div className="mt-4 space-y-2">{alerts.length ? alerts.map((a) => <div key={a.id} className="rounded-lg border border-slate-800 p-3"><div className="flex items-center justify-between gap-2"><span className="text-xs font-semibold uppercase">{a.severity}</span><span className="text-xs text-slate-500">{a.oltId}</span></div><div className="mt-1 text-sm">{a.message}</div><div className="mt-1 text-xs text-slate-500">{a.onuId || a.ponId || a.code} · {formatTime(a.lastSeen)}</div></div>) : <div className="rounded-lg border border-slate-800 p-4 text-sm text-slate-400">No active alerts.</div>}</div></section>
        </div>
        <section className="mt-6 rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Infrastructure</h2><div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{[["Routers","—"],["Switches","—"],["OLTs",String(states.length)],["PON / ONUs","—"]].map(([name,value]) => <div key={name} className="rounded-lg bg-slate-950 p-4"><div className="text-sm text-slate-400">{name}</div><div className="mt-1 text-2xl font-semibold">{value}</div></div>)}</div></section>
      </section>
    </main>
  );
}
