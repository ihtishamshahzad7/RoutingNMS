"use client";

// Connectivity Monitoring detail (6-page rebuild, item 2, extended by
// 2.2, 2.3, 2.4, 2.5): ICMP up/down + latency history, and -- when
// configured on this device -- Port/Service check status + response-time
// history alongside it, sharing the same 1h/24h/7d range selector, plus a
// Download Report PDF button. Backed by GET /api/v1/ping/{id}/history-range
// and GET /api/v1/port-check/{id}/history-range.
//
// 2.3: raw-probe scatter graphing replaced with aggregation via
// lib/monitoring-aggregate.ts -- 24h buckets into one point per hour,
// 7d buckets into one point per day (average latency + uptime % per
// bucket), 1h stays raw/near-raw. Applied identically to the ICMP chart
// and the Port/Service chart.
//
// 2.5: UI polish -- chart skeletons while loading instead of "Loading…"
// text, an inline error + Retry button instead of a blank chart on fetch
// failure, and empty-state copy that distinguishes "this device has no
// history at all yet" (1h view) from "nothing in this particular range"
// (24h/7d), per the user's specified wording for the former.

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Activity, Plug, Download, RotateCw } from "lucide-react";
import { LineChart, Line, ResponsiveContainer, XAxis, YAxis, Tooltip, CartesianGrid } from "recharts";
import { apiFetch, ApiError } from "../../../../lib/api";
import { EngPanel, EngButton, EngModal, EngField, EngInput } from "../../../../components/ui/engineer";
import { HeartbeatBar, type Beat } from "../../dashboard/HeartbeatBar";
import { aggregate, type RawPoint } from "../../../../lib/monitoring-aggregate";
import { buildReportData, generateReportPDF } from "../../../../lib/monitoring-report";

const ORG = "tenant-1";
type Device = {
  id: string; name: string; address: string;
  icmpEnabled: boolean;
  portCheckEnabled: boolean; portCheckProtocol: string; portCheckPort: number;
  // Item 3.6: only used to tailor the Download Report modal's copy --
  // buildReportData() re-fetches the device itself and decides on its own
  // whether to include the SNMP sections.
  snmpEnabled: boolean;
};
type ProbeResult = { probedAt: string; rttMs?: number | null; lossPct: number; isReachable: boolean };
type PortHistoryPoint = { probedAt: string; latencyMs?: number | null; isReachable: boolean };
type Range = "1h" | "24h" | "7d";

const RANGES: { key: Range; label: string }[] = [
  { key: "1h", label: "1h" },
  { key: "24h", label: "24h" },
  { key: "7d", label: "7d" },
];

type ChartPoint = { t: number; rtt: number | null; uptimePct: number };

function LatencyChart({ data, unit = "ms" }: { data: ChartPoint[]; unit?: string }) {
  return (
    <ResponsiveContainer width="100%" height={220}>
      <LineChart data={data}>
        <CartesianGrid stroke="#EEF1F4" vertical={false} />
        <XAxis
          dataKey="t"
          type="number"
          domain={["dataMin", "dataMax"]}
          tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
          stroke="#5C6B7A"
          tick={{ fontSize: 11, fontWeight: 400, fill: "#5C6B7A" }}
          tickLine={{ stroke: "#DCE1E8" }}
          axisLine={{ stroke: "#DCE1E8" }}
        />
        <YAxis
          stroke="#5C6B7A"
          tick={{ fontSize: 11, fontWeight: 400, fill: "#5C6B7A" }}
          tickLine={{ stroke: "#DCE1E8" }}
          axisLine={{ stroke: "#DCE1E8" }}
          width={40}
        />
        <Tooltip
          contentStyle={{ background: "#FFFFFF", border: "1px solid #DCE1E8", borderRadius: 4, fontSize: 12, fontFamily: "inherit" }}
          labelStyle={{ color: "#1F2A37", fontWeight: 500 }}
          itemStyle={{ color: "#1F2A37" }}
          labelFormatter={(t) => new Date(t as number).toLocaleString()}
          formatter={(v, name, item) => {
            if (name === "rtt") {
              const uptime = (item?.payload as ChartPoint | undefined)?.uptimePct;
              const uptimeSuffix = uptime != null ? ` · ${uptime.toFixed(1)}% up` : "";
              return [v == null ? `timeout${uptimeSuffix}` : `${v} ${unit}${uptimeSuffix}`, "Latency"];
            }
            return [v, name];
          }}
        />
        <Line type="monotone" dataKey="rtt" stroke="#2E7BF6" strokeWidth={1.5} dot={false} isAnimationActive={false} connectNulls={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

/** Animated placeholder shaped like the chart it stands in for, so the
 * layout doesn't jump once data arrives (item 2.5: "skeleton placeholders,
 * not blank space or spinners alone"). */
function ChartSkeleton() {
  return (
    <div className="flex h-[220px] animate-pulse items-end gap-1 px-2 pb-4">
      {[38, 62, 45, 70, 52, 80, 58, 40, 66, 48, 72, 55, 44, 68, 50].map((h, i) => (
        <div key={i} className="flex-1 rounded-t-[2px] bg-[#EEF1F4]" style={{ height: `${h}%` }} />
      ))}
    </div>
  );
}

function HeartbeatSkeleton() {
  return (
    <div className="flex animate-pulse items-end gap-[2px] py-1" style={{ height: 28 }}>
      {Array.from({ length: 50 }).map((_, i) => (
        <span key={i} className="w-[3px] shrink-0 rounded-[1px] bg-[#EEF1F4]" style={{ height: "100%" }} />
      ))}
    </div>
  );
}

function ErrorBlock({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-12 text-center">
      <span className="text-[13px] text-[#C4362D]">{message}</span>
      <button
        onClick={onRetry}
        className="inline-flex items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-3 py-1.5 text-[12px] font-medium text-[#1F2A37] hover:bg-[#F4F6F9]"
      >
        <RotateCw size={12} /> Retry
      </button>
    </div>
  );
}

function EmptyBlock({ range }: { range: Range }) {
  return (
    <div className="py-16 text-center text-[13px] text-[#8A96A3]">
      {range === "1h" ? "No data yet — first results appear within one check interval." : "No data in this range — try a shorter range, or check back later."}
    </div>
  );
}

export default function ConnectivityMonitoringDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [device, setDevice] = useState<Device | null>(null);
  const [range, setRange] = useState<Range>("24h");

  const [reportOpen, setReportOpen] = useState(false);
  const [reportFrom, setReportFrom] = useState(() => {
    const d = new Date();
    d.setDate(d.getDate() - 7);
    return d.toISOString().slice(0, 10);
  });
  const [reportTo, setReportTo] = useState(() => new Date().toISOString().slice(0, 10));
  const [reportBusy, setReportBusy] = useState(false);
  const [reportError, setReportError] = useState("");

  async function downloadReport() {
    setReportBusy(true);
    setReportError("");
    try {
      const from = new Date(`${reportFrom}T00:00:00`);
      const to = new Date(`${reportTo}T23:59:59.999`);
      if (to.getTime() <= from.getTime()) {
        setReportError("End date must be after start date.");
        return;
      }
      const data = await buildReportData(id, from, to);
      await generateReportPDF(data);
      setReportOpen(false);
    } catch (e) {
      setReportError(e instanceof ApiError ? e.message : "Unable to generate the report.");
    } finally {
      setReportBusy(false);
    }
  }

  const [history, setHistory] = useState<ProbeResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [portHistory, setPortHistory] = useState<PortHistoryPoint[]>([]);
  const [portLoading, setPortLoading] = useState(true);
  const [portError, setPortError] = useState("");

  useEffect(() => {
    let active = true;
    apiFetch<Device[]>(`/devices?organizationId=${ORG}`)
      .then((ds) => { if (active) setDevice(ds.find((d) => d.id === id) ?? null); })
      .catch(() => {});
    return () => { active = false; };
  }, [id]);

  const loadIcmp = useCallback(() => {
    let active = true;
    setLoading(true);
    apiFetch<{ history: ProbeResult[] }>(`/ping/${id}/history-range?range=${range}`)
      .then((r) => { if (active) { setHistory(r.history); setError(""); } })
      .catch((e) => { if (active) setError(e instanceof ApiError ? e.message : "Unable to load ICMP history."); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [id, range]);

  useEffect(() => loadIcmp(), [loadIcmp]);

  const loadPort = useCallback(() => {
    if (!device?.portCheckEnabled) { setPortHistory([]); setPortLoading(false); return () => {}; }
    let active = true;
    setPortLoading(true);
    apiFetch<{ history: PortHistoryPoint[] }>(`/port-check/${id}/history-range?range=${range}`)
      .then((r) => { if (active) { setPortHistory(r.history); setPortError(""); } })
      .catch((e) => { if (active) setPortError(e instanceof ApiError ? e.message : "Unable to load Port/Service check history."); })
      .finally(() => { if (active) setPortLoading(false); });
    return () => { active = false; };
  }, [id, range, device?.portCheckEnabled]);

  useEffect(() => loadPort(), [loadPort]);

  const aggregatedIcmp = useMemo(() => {
    const raw: RawPoint[] = history.map((p) => ({
      t: new Date(p.probedAt).getTime(),
      latencyMs: p.rttMs ?? null,
      reachable: p.isReachable,
    }));
    return aggregate(raw, range);
  }, [history, range]);

  const chartData: ChartPoint[] = useMemo(
    () => aggregatedIcmp.map((p) => ({ t: p.t, rtt: p.avgLatencyMs, uptimePct: p.uptimePct })),
    [aggregatedIcmp]
  );
  const beats: Beat[] = useMemo(
    () => aggregatedIcmp.map((p) => ({ reachable: p.reachable, lossPct: 100 - p.uptimePct, probedAt: new Date(p.t).toISOString() })),
    [aggregatedIcmp]
  );
  const uptimePct = useMemo(() => {
    if (!history.length) return null;
    return (history.filter((p) => p.isReachable).length / history.length) * 100;
  }, [history]);

  const aggregatedPort = useMemo(() => {
    const raw: RawPoint[] = portHistory.map((p) => ({
      t: new Date(p.probedAt).getTime(),
      latencyMs: p.latencyMs ?? null,
      reachable: p.isReachable,
    }));
    return aggregate(raw, range);
  }, [portHistory, range]);

  const portChartData: ChartPoint[] = useMemo(
    () => aggregatedPort.map((p) => ({ t: p.t, rtt: p.avgLatencyMs, uptimePct: p.uptimePct })),
    [aggregatedPort]
  );
  const portBeats: Beat[] = useMemo(
    () => aggregatedPort.map((p) => ({ reachable: p.reachable, lossPct: 100 - p.uptimePct, probedAt: new Date(p.t).toISOString() })),
    [aggregatedPort]
  );
  const portUptimePct = useMemo(() => {
    if (!portHistory.length) return null;
    return (portHistory.filter((p) => p.isReachable).length / portHistory.length) * 100;
  }, [portHistory]);
  const portCurrentStatus = portHistory.length ? portHistory[portHistory.length - 1].isReachable : null;

  return (
    <main className="mx-auto max-w-5xl px-6 py-6">
      <Link href="/icmp-monitoring" className="mb-3 inline-flex items-center gap-1.5 text-[12px] text-[#5C6B7A] hover:text-[#1F2A37]">
        <ArrowLeft size={13} /> Back to Connectivity Monitoring
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
          <EngButton onClick={() => setReportOpen(true)}>
            <Download size={14} /> Download Report
          </EngButton>
        </div>
      </div>

      {reportOpen && (
        <EngModal
          title="Download Report"
          subtitle="PDF covering ICMP and Port/Service checks for the selected date range"
          onClose={() => (reportBusy ? null : setReportOpen(false))}
          footer={
            <>
              <EngButton onClick={() => setReportOpen(false)} disabled={reportBusy}>
                Cancel
              </EngButton>
              <EngButton variant="primary" onClick={downloadReport} disabled={reportBusy}>
                {reportBusy ? "Generating…" : "Generate PDF"}
              </EngButton>
            </>
          }
        >
          <div className="grid grid-cols-2 gap-3">
            <EngField label="From">
              <EngInput type="date" value={reportFrom} max={reportTo} onChange={(e) => setReportFrom(e.target.value)} />
            </EngField>
            <EngField label="To">
              <EngInput type="date" value={reportTo} min={reportFrom} onChange={(e) => setReportTo(e.target.value)} />
            </EngField>
          </div>
          {reportError && <p className="mt-3 text-[12px] text-[#C4362D]">{reportError}</p>}
          <p className="mt-3 text-[11px] text-[#8A96A3]">
            The report lists overall uptime %, average latency/response time, and every up/down event for each
            configured check type. Time spent in an active maintenance window is excluded from downtime and the
            uptime % calculation.
            {device?.snmpEnabled && " This device also has SNMP monitoring enabled, so the report also includes port up/down history and CPU/memory threshold alerts."}
          </p>
        </EngModal>
      )}

      <EngPanel title={`ICMP · Latency (RTT ms)${range === "1h" ? "" : range === "24h" ? " · hourly avg" : " · daily avg"}`}>
        {loading ? (
          <ChartSkeleton />
        ) : error ? (
          <ErrorBlock message={error} onRetry={loadIcmp} />
        ) : chartData.length === 0 ? (
          <EmptyBlock range={range} />
        ) : (
          <LatencyChart data={chartData} />
        )}
      </EngPanel>

      <EngPanel title="ICMP · Up / down" className="mt-4">
        {loading ? (
          <HeartbeatSkeleton />
        ) : error ? (
          <ErrorBlock message={error} onRetry={loadIcmp} />
        ) : beats.length === 0 ? (
          <EmptyBlock range={range} />
        ) : (
          <div className="overflow-x-auto py-1">
            <HeartbeatBar beats={beats} height={28} />
          </div>
        )}
      </EngPanel>

      {device?.portCheckEnabled && (
        <>
          <div className="mb-2 mt-6 flex items-center justify-between">
            <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
              <Plug size={13} /> Port/Service Check · {device.portCheckProtocol.toUpperCase()}
              {device.portCheckProtocol !== "tcp" ? "" : ` :${device.portCheckPort}`}
            </div>
            {portUptimePct != null && (
              <span className="font-mono text-[13px] text-[#1F2A37]">
                {portCurrentStatus != null && (
                  <span className={portCurrentStatus ? "text-[#1E8E5A]" : "text-[#C4362D]"}>{portCurrentStatus ? "● up" : "● down"}</span>
                )}{" "}
                {portUptimePct.toFixed(2)}% up <span className="text-[#8A96A3]">({range})</span>
              </span>
            )}
          </div>

          <EngPanel title={`Response time (${device.portCheckProtocol === "tcp" ? "connect ms" : "ms"})${range === "1h" ? "" : range === "24h" ? " · hourly avg" : " · daily avg"}`}>
            {portLoading ? (
              <ChartSkeleton />
            ) : portError ? (
              <ErrorBlock message={portError} onRetry={loadPort} />
            ) : portChartData.length === 0 ? (
              <EmptyBlock range={range} />
            ) : (
              <LatencyChart data={portChartData} />
            )}
          </EngPanel>

          <EngPanel title="Port/Service · Up / down" className="mt-4">
            {portLoading ? (
              <HeartbeatSkeleton />
            ) : portError ? (
              <ErrorBlock message={portError} onRetry={loadPort} />
            ) : portBeats.length === 0 ? (
              <EmptyBlock range={range} />
            ) : (
              <div className="overflow-x-auto py-1">
                <HeartbeatBar beats={portBeats} height={28} />
              </div>
            )}
          </EngPanel>
        </>
      )}
    </main>
  );
}
