"use client";

// ICMP Monitoring detail (6-page rebuild, item 2): a graph of one device's
// ICMP up/down + latency history, with a 1h/24h/7d range selector, backed
// by the new GET /api/v1/ping/{id}/history-range?range=... endpoint
// (backend/internal/ping/api.go, poller.go's Repository.HistoryRange).

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Activity } from "lucide-react";
import { LineChart, Line, ResponsiveContainer, XAxis, YAxis, Tooltip, CartesianGrid } from "recharts";
import { apiFetch, ApiError } from "../../../../lib/api";
import { EngPanel } from "../../../../components/ui/engineer";
import { HeartbeatBar, type Beat } from "../../dashboard/HeartbeatBar";

const ORG = "tenant-1";
type Device = { id: string; name: string; address: string };
type ProbeResult = { probedAt: string; rttMs?: number | null; lossPct: number; isReachable: boolean };
type Range = "1h" | "24h" | "7d";

const RANGES: { key: Range; label: string }[] = [
  { key: "1h", label: "1h" },
  { key: "24h", label: "24h" },
  { key: "7d", label: "7d" },
];

export default function IcmpMonitoringDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [device, setDevice] = useState<Device | null>(null);
  const [range, setRange] = useState<Range>("24h");
  const [history, setHistory] = useState<ProbeResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    apiFetch<Device[]>(`/devices?organizationId=${ORG}`)
      .then((ds) => { if (active) setDevice(ds.find((d) => d.id === id) ?? null); })
      .catch(() => {});
    return () => { active = false; };
  }, [id]);

  useEffect(() => {
    let active = true;
    setLoading(true);
    apiFetch<{ history: ProbeResult[] }>(`/ping/${id}/history-range?range=${range}`)
      .then((r) => { if (active) { setHistory(r.history); setError(""); } })
      .catch((e) => { if (active) setError(e instanceof ApiError ? e.message : "Unable to load ICMP history."); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [id, range]);

  const chartData = useMemo(
    () => history.map((p) => ({ t: new Date(p.probedAt).getTime(), rtt: p.rttMs ?? null })),
    [history]
  );
  const beats: Beat[] = useMemo(
    () => history.map((p) => ({ reachable: p.isReachable, lossPct: p.lossPct, probedAt: p.probedAt })),
    [history]
  );
  const uptimePct = useMemo(() => {
    if (!history.length) return null;
    return (history.filter((p) => p.isReachable).length / history.length) * 100;
  }, [history]);

  return (
    <main className="mx-auto max-w-5xl px-6 py-6">
      <Link href="/icmp-monitoring" className="mb-3 inline-flex items-center gap-1.5 text-[12px] text-[#5C6B7A] hover:text-[#1F2A37]">
        <ArrowLeft size={13} /> Back to ICMP Monitoring
      </Link>
      <div className="mb-4 flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
            <Activity size={13} /> Monitoring
          </div>
          <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">{device?.name ?? "Device"}</h1>
          <p className="mt-1 font-mono text-[13px] text-[#5C6B7A]">{device?.address ?? id}</p>
        </div>
        <div className="flex items-center gap-3">
          {uptimePct != null && (
            <span className="font-mono text-[13px] text-[#1F2A37]">{uptimePct.toFixed(2)}% up <span className="text-[#8A96A3]">({range})</span></span>
          )}
          <div className="flex rounded-[4px] border border-[#DCE1E8] bg-white p-0.5">
            {RANGES.map((r) => (
              <button
                key={r.key}
                onClick={() => setRange(r.key)}
                className={`rounded-[3px] px-3 py-1 text-[12px] font-medium transition-colors duration-150 ${
                  range === r.key ? "bg-[#2E7BF6] text-white" : "text-[#5C6B7A] hover:bg-[#F4F6F9]"
                }`}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      <EngPanel title="Latency (RTT ms)">
        {loading ? (
          <div className="py-16 text-center text-[13px] text-[#8A96A3]">Loading…</div>
        ) : error ? (
          <div className="py-16 text-center text-[13px] text-[#C4362D]">{error}</div>
        ) : chartData.length === 0 ? (
          <div className="py-16 text-center text-[13px] text-[#8A96A3]">No ICMP probes recorded in this window yet.</div>
        ) : (
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <CartesianGrid stroke="#EEF1F4" vertical={false} />
              <XAxis
                dataKey="t"
                type="number"
                domain={["dataMin", "dataMax"]}
                tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                stroke="#8A96A3"
                fontSize={11}
              />
              <YAxis stroke="#8A96A3" fontSize={11} width={40} />
              <Tooltip
                contentStyle={{ background: "#FFFFFF", border: "1px solid #DCE1E8", borderRadius: 4, fontSize: 12 }}
                labelFormatter={(t) => new Date(t as number).toLocaleString()}
                formatter={(v) => [v == null ? "timeout" : `${v} ms`, "RTT"]}
              />
              <Line type="monotone" dataKey="rtt" stroke="#2E7BF6" strokeWidth={1.5} dot={false} isAnimationActive={false} connectNulls={false} />
            </LineChart>
          </ResponsiveContainer>
        )}
      </EngPanel>

      <EngPanel title="Up / down" className="mt-4">
        {beats.length === 0 ? (
          <div className="py-6 text-center text-[13px] text-[#8A96A3]">No probes in this window yet.</div>
        ) : (
          <div className="overflow-x-auto py-1">
            <HeartbeatBar beats={beats} height={28} />
          </div>
        )}
      </EngPanel>
    </main>
  );
}
