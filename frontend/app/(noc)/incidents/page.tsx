"use client";

// Screen — Incidents. Feature 0.6 (Full UI/UX Design Spec): this page was
// on AGENTS.md's own "light token pass still pending" list (confirmed by
// reading that file) and, unlike the other pages on that list, was
// actually broken rather than merely off-palette -- it used shadcn-style
// classes (text-muted-foreground, bg-background, bg-primary,
// text-primary-foreground) that app/globals.css's @theme never defines,
// so they silently rendered as unstyled/inherited color instead of the
// intended muted gray. It also wrapped its own `min-h-screen` <main>,
// redundant (and slightly wrong) inside the shared NocLayout's own
// overflow-y-auto scroll container every other page already relies on
// (see app/(noc)/layout.tsx). Fixed here onto the real GitHub-dark tokens
// and components/ui/* primitives, matching every other core screen (e.g.
// topology/page.tsx's `mx-auto max-w-7xl px-6 py-6` outer wrapper). No
// data-fetching, endpoint, or behavior change -- same /api/incidents
// calls, same polling-free load-on-filter-change logic, same actions.
import { useEffect, useMemo, useState } from "react";
import { apiFetch } from "../../../lib/api";
import { PageHeader } from "../../../components/ui/form";
import { StatCard } from "../../../components/ui/card";
import { StatusPill } from "../../../components/ui/status-pill";
import { Button } from "../../../components/ui/primitives";

type Incident = {
  id: string;
  status: string;
  severity: string;
  title: string;
  source: string;
  resourceId: string;
  startedAt: string;
  acknowledgedAt?: string;
  resolvedAt?: string;
};

export default function IncidentsPage() {
  const [items, setItems] = useState<Incident[]>([]);
  const [status, setStatus] = useState("");
  const [severity, setSeverity] = useState("");
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const q = new URLSearchParams();
      if (status) q.set("status", status);
      if (severity) q.set("severity", severity);
      setItems(await apiFetch<Incident[]>(`/api/incidents?${q}`));
    } catch {
      /* keep last list */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, severity]);

  const counts = useMemo(
    () => ({
      critical: items.filter((i) => i.severity === "critical" && i.status !== "resolved").length,
      open: items.filter((i) => i.status === "open").length,
      ack: items.filter((i) => i.status === "acknowledged").length,
    }),
    [items]
  );

  const action = async (id: string, a: "acknowledge" | "resolve") => {
    await apiFetch(`/api/incidents/${encodeURIComponent(id)}/${a}`, { method: "POST" });
    load();
  };

  return (
    <main className="mx-auto max-w-7xl px-6 py-6">
      <PageHeader eyebrow="Network Operations Center" title="Incidents" description="Live operational events requiring attention." />

      <section className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard label="Critical" value={counts.critical} accent="text-status-crit" />
        <StatCard label="Open" value={counts.open} accent="text-status-warn" />
        <StatCard label="Acknowledged" value={counts.ack} />
      </section>

      <section className="mb-4 flex flex-wrap gap-3">
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">All statuses</option>
          <option value="open">Open</option>
          <option value="acknowledged">Acknowledged</option>
          <option value="resolved">Resolved</option>
        </select>
        <select className="input" value={severity} onChange={(e) => setSeverity(e.target.value)}>
          <option value="">All severities</option>
          <option value="critical">Critical</option>
          <option value="warning">Warning</option>
          <option value="info">Info</option>
        </select>
      </section>

      <section className="overflow-hidden rounded-[8px] border border-border">
        <div className="overflow-x-auto">
          <table className="tbl w-full text-sm">
            <thead>
              <tr>
                <th className="p-4 text-left">Incident</th>
                <th className="p-4 text-left">Severity</th>
                <th className="p-4 text-left">Status</th>
                <th className="p-4 text-left">Resource</th>
                <th className="p-4 text-left">Started</th>
                <th className="p-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-text-muted">
                    Loading incidents…
                  </td>
                </tr>
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={6} className="p-8 text-center text-text-muted">
                    No incidents found.
                  </td>
                </tr>
              ) : (
                items.map((i) => (
                  <tr key={i.id} className="border-t border-border">
                    <td className="p-4">
                      <div className="font-medium text-text-primary">{i.title}</div>
                      <div className="text-xs text-text-muted">
                        {i.source} · {i.id}
                      </div>
                    </td>
                    <td className="p-4">
                      <StatusPill status={i.severity} />
                    </td>
                    <td className="p-4">
                      <StatusPill status={i.status} />
                    </td>
                    <td className="p-4 font-mono text-xs text-text-muted">{i.resourceId}</td>
                    <td className="p-4 text-text-muted">{new Date(i.startedAt).toLocaleString()}</td>
                    <td className="space-x-2 p-4 text-right">
                      {i.status === "open" && (
                        <Button variant="secondary" onClick={() => action(i.id, "acknowledge")}>
                          Acknowledge
                        </Button>
                      )}
                      {i.status !== "resolved" && (
                        <Button variant="primary" onClick={() => action(i.id, "resolve")}>
                          Resolve
                        </Button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}
