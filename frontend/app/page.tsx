"use client";

import { useEffect, useState } from "react";

type Stat = { label: string; value: string; detail: string };

const stats: Stat[] = [
  { label: "Network Health", value: "100%", detail: "All monitored services" },
  { label: "Devices Online", value: "0", detail: "0 monitored" },
  { label: "Critical Alerts", value: "0", detail: "No active incidents" },
  { label: "Open Incidents", value: "0", detail: "No active incidents" },
];

export default function Home() {
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    fetch("/api/health")
      .then((response) => setConnected(response.ok))
      .catch(() => setConnected(false));
  }, []);

  return (
    <main className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800 px-6 py-4">
        <div className="mx-auto flex max-w-7xl items-center justify-between">
          <div><div className="text-xl font-bold">RoutingNMS</div><div className="text-xs text-slate-400">Network Operations Center</div></div>
          <div className="flex items-center gap-2 text-xs text-slate-300"><span className={`h-2 w-2 rounded-full ${connected ? "bg-emerald-400" : "bg-amber-400"}`} />{connected ? "Backend connected" : "Backend pending"}</div>
        </div>
      </header>
      <section className="mx-auto max-w-7xl px-6 py-8">
        <div className="mb-8"><p className="text-sm font-medium text-cyan-400">LIVE NOC</p><h1 className="mt-1 text-3xl font-bold">Network Overview</h1><p className="mt-2 text-sm text-slate-400">Real-time visibility for routers, switches, OLTs and access infrastructure.</p></div>
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {stats.map((stat) => <article key={stat.label} className="rounded-xl border border-slate-800 bg-slate-900 p-5"><div className="text-sm text-slate-400">{stat.label}</div><div className="mt-2 text-3xl font-semibold">{stat.value}</div><div className="mt-2 text-xs text-slate-500">{stat.detail}</div></article>)}
        </div>
        <div className="mt-6 grid gap-6 lg:grid-cols-3">
          <section className="rounded-xl border border-slate-800 bg-slate-900 p-5 lg:col-span-2"><div className="flex justify-between"><h2 className="font-semibold">Network Activity</h2><span className="text-xs text-slate-500">Live events</span></div><div className="mt-8 flex min-h-48 items-center justify-center rounded-lg border border-dashed border-slate-800 text-sm text-slate-500">Monitoring data will appear here when devices are registered.</div></section>
          <section className="rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Active Alerts</h2><div className="mt-5 rounded-lg border border-slate-800 p-4 text-sm text-slate-400">No active alerts.</div></section>
        </div>
        <section className="mt-6 rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Infrastructure</h2><div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{["Routers", "Switches", "OLTs", "PON / ONUs"].map((name) => <div key={name} className="rounded-lg bg-slate-950 p-4"><div className="text-sm text-slate-400">{name}</div><div className="mt-1 text-2xl font-semibold">0</div></div>)}</div></section>
      </section>
    </main>
  );
}
