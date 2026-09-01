"use client";

// Screen — Topology. Restyled onto the GitHub-dark design system + new D3
// force-directed canvas (components/topology-force.tsx). Data still comes
// from the real backend: persisted graph (/api/topology), discovery status
// and manual rediscovery (/api/v1/topology/*), synced via apiFetch.
import { useEffect, useMemo, useState } from "react";
import { apiFetch } from "../../../lib/api";
import { Card, StatCard } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { StatusPill } from "../../../components/ui/status-pill";
import { TopologyForce, type TopoNode, type TopoLink } from "../../../components/topology-force";

type Graph = { nodes: TopoNode[]; links: TopoLink[]; generatedAt: string };
type DiscoveryStatus = { lastRun?: string; lastError?: string; links?: number; nodes?: number; running?: boolean; interval?: number };

export default function TopologyPage() {
  const [graph, setGraph] = useState<Graph>({ nodes: [], links: [], generatedAt: "" });
  const [status, setStatus] = useState<DiscoveryStatus>({});
  const [error, setError] = useState("");
  const [discovering, setDiscovering] = useState(false);

  const loadStatus = async () => {
    try { setStatus(await apiFetch<DiscoveryStatus>("/topology/status")); } catch { /* keep last */ }
  };
  const load = async () => {
    try {
      const data = await apiFetch<Graph>("/api/topology");
      setGraph(data); setError("");
    } catch (e) { setError(e instanceof Error ? e.message : "Topology unavailable"); }
  };

  useEffect(() => {
    let live = true;
    const run = async () => { await Promise.all([load(), loadStatus()]); };
    run();
    const t = setInterval(() => { if (live) { load(); loadStatus(); } }, 10000);
    return () => { live = false; clearInterval(t); };
  }, []);

  const rediscover = async () => {
    setDiscovering(true);
    try {
      await apiFetch("/topology/discover", { method: "POST" });
      await Promise.all([load(), loadStatus()]);
    } catch { /* engine may already be running */ }
    finally { setDiscovering(false); }
  };

  const stats = useMemo(() => ({
    up: graph.links.filter(x => x.status === "up").length,
    down: graph.links.filter(x => x.status === "down").length,
    degraded: graph.links.filter(x => x.status === "degraded").length,
  }), [graph.links]);

  return (
    <main className="mx-auto max-w-7xl px-6 py-6">
      <div className="mb-6">
        <div className="label text-[#8B949E]">Network Map</div>
        <h1 className="mt-1 text-[22px] font-bold tracking-[-0.5px] text-[#E6EDF3]">Topology</h1>
        <p className="mt-1 text-xs text-[#8B949E]">Discovered network relationships and link health (LLDP discovery + ICMP probe results).</p>
      </div>

      <div className="mb-6 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <StatCard label="Devices" value={graph.nodes.length} accent="text-[#58A6FF]" sub="Nodes in persisted graph" />
        <StatCard label="Links" value={graph.links.length} accent="text-[#58A6FF]" sub="LLDP-discovered edges" />
        <StatCard label="Up" value={stats.up} accent="text-[#3FB950]" sub="Healthy links" />
        <StatCard label="Issues" value={stats.down + stats.degraded} accent={stats.down + stats.degraded ? "text-[#F78166]" : "text-[#3FB950]"} sub="Down / degraded links" />
      </div>

      {error && <div className="mb-4 rounded-[5px] border border-[#672525] bg-[#2D1212] p-3 text-xs text-[#F78166]">{error}</div>}

      <Card title="Live topology" headerRight={
        <Button variant="secondary" onClick={rediscover} disabled={discovering || status.running}>
          {discovering ? "Discovering…" : "Rediscover now"}
        </Button>
      } className="mb-6">
        <div className="flex flex-wrap items-center gap-4 border-b border-[#21262D] px-4 py-2.5 text-[10px] text-[#8B949E]">
          {status.running
            ? <span className="inline-flex items-center gap-1.5"><span className="dot dot-up dot-pulse" /><span className="text-[#3FB950]">Discovery running</span></span>
            : <span>Auto-refresh: 10s</span>}
          {status.lastRun && <span>Last discovery {timeAgo(status.lastRun)}</span>}
          {typeof status.links === "number" && <span>{status.links} links found</span>}
          {typeof status.nodes === "number" && <span>{status.nodes} nodes found</span>}
          {status.lastError && <span className="text-[#F78166]">Error: {status.lastError}</span>}
        </div>
        <div className="p-4">
          <TopologyForce nodes={graph.nodes} links={graph.links} />
        </div>
      </Card>

      <Card title={`Discovered links (${graph.links.length})`}>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs text-[#C9D1D9]">
            <thead>
              <tr className="border-b border-[#21262D] text-[#8B949E]">
                <th className="px-4 py-2.5 font-medium">Source</th>
                <th className="px-4 py-2.5 font-medium">Target</th>
                <th className="px-4 py-2.5 font-medium">Status</th>
                <th className="px-4 py-2.5 font-medium">Latency</th>
                <th className="px-4 py-2.5 font-medium">Loss</th>
              </tr>
            </thead>
            <tbody>
              {graph.links.map(l => (
                <tr key={l.id} className="border-b border-[#1c2128]">
                  <td className="px-4 py-3 font-mono text-[#8B949E]">{l.source}</td>
                  <td className="px-4 py-3 font-mono text-[#8B949E]">{l.target}</td>
                  <td className="px-4 py-3"><StatusPill status={l.status} label={l.status} pulse={l.status === "down"} /></td>
                  <td className="px-4 py-3 text-[#8B949E]">{l.latencyMs ?? 0} ms</td>
                  <td className="px-4 py-3 text-[#8B949E]">{l.packetLossPct ?? 0}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </main>
  );
}

function timeAgo(iso: string) {
  try {
    const d = +new Date(iso);
    if (!d) return "";
    const s = Math.max(0, Math.floor((Date.now() - d) / 1000));
    if (s < 5) return "just now";
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.floor(s / 60)}m ago`;
    return `${Math.floor(s / 3600)}h ago`;
  } catch { return ""; }
}