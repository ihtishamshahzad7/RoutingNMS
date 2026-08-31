"use client";

import { useEffect, useMemo, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type SyslogRecord = {
  id: number;
  receivedAt: string;
  sourceIp: string;
  facility?: number;
  severity?: number;
  hostname?: string;
  tag?: string;
  message: string;
};

// RFC 3164 severity levels (0 = most severe).
const SEVERITY_LABELS: Record<number, string> = {
  0: "emergency", 1: "alert", 2: "critical", 3: "error",
  4: "warning", 5: "notice", 6: "info", 7: "debug",
};

function severityBadge(sev?: number) {
  if (sev === undefined) return "bg-slate-800 text-slate-400";
  if (sev <= 2) return "bg-red-950 text-red-300";
  if (sev === 3) return "bg-amber-950 text-amber-300";
  if (sev <= 5) return "bg-cyan-950 text-cyan-300";
  return "bg-slate-800 text-slate-400";
}

export default function SyslogPage() {
  const [items, setItems] = useState<SyslogRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [hostFilter, setHostFilter] = useState("");
  const [maxSeverity, setMaxSeverity] = useState("");

  async function load() {
    setLoading(true); setError("");
    try {
      const q = new URLSearchParams({ limit: "200" });
      if (hostFilter) q.set("host", hostFilter);
      if (maxSeverity !== "") q.set("maxSeverity", maxSeverity);
      setItems(await apiFetch<SyslogRecord[]>(`/syslog?${q}`));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Unable to load syslog messages.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 10000);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hostFilter, maxSeverity]);

  const hosts = useMemo(() => Array.from(new Set(items.map(i => i.sourceIp))).sort(), [items]);

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-widest text-cyan-400">LIVE NOC</div>
        <h1 className="mt-2 text-3xl font-bold">Syslog</h1>
        <p className="mt-2 text-sm text-slate-400">
          Point your OLTs, routers, switches and CMTS gear at this NMS as a syslog target to see their messages here.
          Listening on UDP/TCP <code className="rounded bg-slate-900 px-1.5 py-0.5">1514</code> by default
          (set <code className="rounded bg-slate-900 px-1.5 py-0.5">SYSLOG_ADDR=:514</code> to use the standard port).
        </p>
      </div>

      {error && <div className="mb-5 rounded-lg border border-red-900 bg-red-950/40 px-4 py-3 text-sm text-red-300">{error}</div>}

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <select value={hostFilter} onChange={e => setHostFilter(e.target.value)} className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm">
          <option value="">All sources</option>
          {hosts.map(h => <option key={h} value={h}>{h}</option>)}
        </select>
        <select value={maxSeverity} onChange={e => setMaxSeverity(e.target.value)} className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm">
          <option value="">All severities</option>
          {Object.entries(SEVERITY_LABELS).map(([v, label]) => <option key={v} value={v}>{label} and worse</option>)}
        </select>
        <button onClick={load} className="rounded border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button>
      </div>

      <section className="rounded-xl border border-slate-800 bg-slate-900 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-800 text-xs text-slate-500">
              <tr>
                <th className="px-3 py-3">Received</th>
                <th className="px-3 py-3">Source</th>
                <th className="px-3 py-3">Severity</th>
                <th className="px-3 py-3">Tag</th>
                <th className="px-3 py-3">Message</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">Loading…</td></tr>
              ) : items.length ? items.map(i => (
                <tr key={i.id} className="border-b border-slate-800/70 align-top">
                  <td className="whitespace-nowrap px-3 py-3 text-xs text-slate-400">{new Date(i.receivedAt).toLocaleString()}</td>
                  <td className="px-3 py-3 font-mono text-xs">{i.hostname || i.sourceIp}</td>
                  <td className="px-3 py-3">
                    <span className={`rounded-full px-2 py-1 text-xs ${severityBadge(i.severity)}`}>
                      {i.severity !== undefined ? SEVERITY_LABELS[i.severity] ?? i.severity : "—"}
                    </span>
                  </td>
                  <td className="px-3 py-3 text-xs text-slate-400">{i.tag || "—"}</td>
                  <td className="px-3 py-3 text-slate-200">{i.message}</td>
                </tr>
              )) : (
                <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">No syslog messages received yet.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}
