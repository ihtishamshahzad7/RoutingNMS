"use client";

// Screen 4 — AI Incident Hub with RCA report. Restyled onto the GitHub-dark
// design tokens (Card, StatCard, StatusPill, Button, AiBadge). Behavior is
// unchanged and additive to the backend: list + ack/resolve through the real
// /api/v1/alerts/incidents API.
import { useEffect, useState, type ReactNode } from "react";
import { apiFetch } from "../../../lib/api";
import { Card, StatCard } from "../../../components/ui/card";
import { StatusPill } from "../../../components/ui/status-pill";
import { Button, AiBadge } from "../../../components/ui/primitives";

type AiIncident = {
  id: number; incidentRef: string; status: string; severity: string; title: string; source: string; resourceId: string; deviceId?: number; triggeredAt: string;
  rootCause?: string; confidencePct?: number; affectedServices?: string[]; recommendedActions?: string[]; estimatedImpact?: string; timeline?: { t: string; event: string; detail?: string }[]; rcaCompletedAt?: string;
};

export default function IncidentHubPage() {
  const [items, setItems] = useState<AiIncident[]>([]);
  const [selected, setSelected] = useState<AiIncident | null>(null);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  const load = async () => {
    try {
      const q = status ? `?status=${status}` : "";
      const data = await apiFetch<AiIncident[]>(`/alerts/incidents${q}`);
      setItems(data); setError("");
    } catch (e) { setError(e instanceof Error ? e.message : "Incident hub unavailable"); }
  };
  useEffect(() => { load(); }, [status]);

  const open = async (id: number) => {
    try {
      const data = await apiFetch<AiIncident>(`/alerts/incidents/${id}`);
      setSelected(data);
    } catch { /* keep current selection */ }
  };
  const act = async (id: number, action: "acknowledge" | "resolve") => {
    await apiFetch(`/alerts/incidents/${id}/${action}`, { method: "POST" });
    load(); if (selected) open(selected.id);
  };

  const counts = {
    critical: items.filter(i => i.severity === "critical" && i.status !== "resolved").length,
    open: items.filter(i => i.status === "open" || i.status === "analyzing").length,
    resolved: items.filter(i => i.status === "resolved").length,
  };

  return (
    <main className="mx-auto max-w-7xl px-6 py-6">
      <div className="mb-6">
        <div className="label text-[#8B949E]">Incident Management</div>
        <h1 className="mt-1 text-[22px] font-bold tracking-[-0.5px] text-[#E6EDF3]">Incident Hub</h1>
        <p className="mt-1 text-xs text-[#8B949E]">Durable incident history with AI root-cause analysis.</p>
      </div>

      {error && <div className="mb-4 rounded-[5px] border border-[#672525] bg-[#2D1212] p-3 text-xs text-[#F78166]">{error}</div>}

      <div className="mb-6 grid gap-3 md:grid-cols-3">
        <StatCard label="Critical" value={counts.critical} accent={counts.critical ? "text-[#F78166]" : "text-[#3FB950]"} sub="Unresolved critical incidents" />
        <StatCard label="Open" value={counts.open} accent={counts.open ? "text-[#D29922]" : "text-[#3FB950]"} sub="Open / analyzing" />
        <StatCard label="Resolved" value={counts.resolved} accent="text-[#3FB950]" sub="Closed incidents" />
      </div>

      <div className="mb-5 flex items-center gap-3">
        <select
          className="rounded-[5px] border border-[#30363D] bg-[#161B22] px-3 py-1.5 text-xs text-[#E6EDF3] outline-none focus:border-[#58A6FF]"
          value={status} onChange={e => setStatus(e.target.value)}
        >
          <option value="">All statuses</option><option value="open">Open</option><option value="acknowledged">Acknowledged</option><option value="resolved">Resolved</option><option value="analyzing">Analyzing</option>
        </select>
        <Button variant="secondary" onClick={load}>Refresh</Button>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card title={`Incidents (${items.length})`}>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs text-[#C9D1D9]">
              <thead>
                <tr className="border-b border-[#21262D] text-[#8B949E]">
                  <th className="px-4 py-2.5 font-medium">Incident</th>
                  <th className="px-4 py-2.5 font-medium">Severity</th>
                  <th className="px-4 py-2.5 font-medium">Status</th>
                  <th className="px-4 py-2.5 font-medium">RCA</th>
                  <th className="px-4 py-2.5 font-medium">Triggered</th>
                </tr>
              </thead>
              <tbody>
                {items.length === 0 ? (
                  <tr><td colSpan={5} className="p-8 text-center text-[#484F58]">No incidents yet.</td></tr>
                ) : items.map(i => (
                  <tr key={i.id} className="cursor-pointer border-b border-[#1c2128] transition-colors hover:bg-[#1c2128]" onClick={() => open(i.id)}>
                    <td className="px-4 py-3">
                      <div className="font-medium text-[#E6EDF3] flex items-center gap-1.5">{i.title}<AiBadge /></div>
                      <div className="mt-0.5 text-[10px] text-[#8B949E]">{i.source} · {i.resourceId}</div>
                    </td>
                    <td className="px-4 py-3"><StatusPill status={i.severity} label={i.severity} /></td>
                    <td className="px-4 py-3"><StatusPill status={i.status} label={i.status} pulse={i.status === "analyzing"} /></td>
                    <td className="px-4 py-3">{i.confidencePct != null ? <span className="text-[#58A6FF]">{i.confidencePct}%</span> : <span className="text-[#484F58]">pending</span>}</td>
                    <td className="px-4 py-3 text-[#8B949E]">{new Date(i.triggeredAt).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
        <RcaPanel incident={selected} onAction={act} />
      </div>
    </main>
  );
}

function RcaPanel({ incident, onAction }: { incident: AiIncident | null; onAction: (id: number, a: "acknowledge" | "resolve") => void }) {
  if (!incident) {
    return (
      <Card className="flex min-h-[280px] items-center justify-center p-8">
        <span className="text-xs text-[#484F58]">Select an incident to view its root-cause analysis.</span>
      </Card>
    );
  }
  return (
    <Card title={
      <span className="flex items-center gap-2">Root-Cause Analysis <AiBadge /></span>
    } headerRight={
      <div className="flex gap-2">
        {incident.status !== "resolved" && <Button variant="secondary" onClick={() => onAction(incident.id, "acknowledge")}>Acknowledge</Button>}
        {incident.status !== "resolved" && <Button variant="primary" onClick={() => onAction(incident.id, "resolve")}>Resolve</Button>}
      </div>
    }>
      <div className="space-y-4 p-5">
        <div>
          <h3 className="text-sm font-semibold text-[#E6EDF3]">{incident.title}</h3>
          <p className="mt-0.5 text-[10px] text-[#8B949E]">#{incident.id} · {incident.source} · {incident.resourceId} · triggered {new Date(incident.triggeredAt).toLocaleString()}</p>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Root cause">{incident.rootCause || "Pending analysis…"}</Field>
          <Field label="Confidence">{incident.confidencePct != null ? `${incident.confidencePct}%` : "—"}</Field>
        </div>
        <Field label="Estimated impact">{incident.estimatedImpact || "—"}</Field>
        <Field label="Affected services">{incident.affectedServices?.join(", ") || "—"}</Field>
        <div>
          <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">Recommended actions</div>
          <ul className="space-y-1.5">
            {incident.recommendedActions?.length ? incident.recommendedActions.map((a, i) => (
              <li key={i} className="rounded-[5px] border border-[#21262D] bg-[#0D1117] p-2.5 text-xs text-[#C9D1D9]">{a}</li>
            )) : <li className="text-xs text-[#484F58]">None</li>}
          </ul>
        </div>
        {incident.timeline?.length ? (
          <div>
            <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">Timeline</div>
            <ul className="space-y-1 text-xs text-[#C9D1D9]">
              {incident.timeline.map((t, i) => (
                <li key={i} className="flex gap-2">
                  <span className="font-mono text-[#8B949E]">{t.t?.slice(11, 19)}</span>
                  <span>{t.event}{t.detail ? ` — ${t.detail}` : ""}</span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
    </Card>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <div><div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">{label}</div><div className="text-xs text-[#C9D1D9]">{children}</div></div>;
}
