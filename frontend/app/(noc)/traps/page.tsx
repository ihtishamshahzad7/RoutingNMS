"use client";

import { useEffect, useState, FormEvent } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Trap = {
  id: number;
  receivedAt: string;
  sourceIp: string;
  snmpVersion: string;
  trapOid: string;
  enterpriseOid?: string;
  genericTrap?: number;
  specificTrap?: number;
  varbinds: { oid: string; type: string; value: string }[];
  matchedRuleId?: number;
  severity: string;
};

type Rule = {
  id: number;
  name: string;
  oidPattern: string;
  severity: string;
  enabled: boolean;
  notifyEmail?: string;
  notifyWebhookUrl?: string;
};

function severityBadge(sev: string) {
  if (sev === "critical") return "bg-red-950 text-red-300";
  if (sev === "warning") return "bg-amber-950 text-amber-300";
  return "bg-cyan-950 text-cyan-300";
}

export default function TrapsPage() {
  const [traps, setTraps] = useState<Trap[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showRuleForm, setShowRuleForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [expanded, setExpanded] = useState<number | null>(null);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [trapsRes, rulesRes] = await Promise.all([
        apiFetch<Trap[]>("/traps?limit=200"),
        apiFetch<Rule[]>("/traps/rules"),
      ]);
      setTraps(trapsRes);
      setRules(rulesRes);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Unable to load SNMP traps.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 10000);
    return () => window.clearInterval(timer);
  }, []);

  async function createRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const d = new FormData(event.currentTarget);
      await apiFetch<Rule>("/traps/rules", {
        method: "POST",
        body: JSON.stringify({
          name: d.get("name"),
          oidPattern: d.get("oidPattern") || "*",
          severity: d.get("severity"),
          enabled: true,
          notifyEmail: d.get("notifyEmail") || undefined,
          notifyWebhookUrl: d.get("notifyWebhookUrl") || undefined,
        }),
      });
      event.currentTarget.reset();
      setShowRuleForm(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create trap rule.");
    } finally {
      setSaving(false);
    }
  }

  async function deleteRule(id: number) {
    try {
      await apiFetch(`/traps/rules/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete trap rule.");
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-widest text-cyan-400">LIVE NOC</div>
        <h1 className="mt-2 text-3xl font-bold">SNMP Traps</h1>
        <p className="mt-2 text-sm text-slate-400">
          Point OLTs, routers, switches and UPS controllers at this NMS as an SNMP trap destination (v1/v2c/v3).
          Listening on UDP <code className="rounded bg-slate-900 px-1.5 py-0.5">1162</code> by default
          (set <code className="rounded bg-slate-900 px-1.5 py-0.5">TRAP_ADDR=:162</code> to use the standard port).
          Traps are matched against the rules below to assign a severity.
        </p>
      </div>

      {error && <div className="mb-5 rounded-lg border border-red-900 bg-red-950/40 px-4 py-3 text-sm text-red-300">{error}</div>}

      <section className="mb-8 rounded-xl border border-slate-800 bg-slate-900">
        <div className="flex items-center justify-between border-b border-slate-800 px-5 py-4">
          <h2 className="text-sm font-semibold text-slate-200">Alert rules</h2>
          <button onClick={() => setShowRuleForm(s => !s)} className="rounded border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">
            {showRuleForm ? "Cancel" : "+ New rule"}
          </button>
        </div>

        {showRuleForm && (
          <form onSubmit={createRule} className="grid grid-cols-1 gap-3 border-b border-slate-800 px-5 py-4 sm:grid-cols-2 lg:grid-cols-5">
            <input name="name" required placeholder="Rule name" className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
            <input name="oidPattern" placeholder="OID pattern (e.g. 1.3.6.1.4.1.9.* or *)" className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
            <select name="severity" className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm">
              <option value="info">info</option>
              <option value="warning">warning</option>
              <option value="critical">critical</option>
            </select>
            <input name="notifyEmail" placeholder="Notify email (optional)" className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
            <input name="notifyWebhookUrl" placeholder="Webhook URL (optional)" className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
            <button disabled={saving} className="rounded-lg bg-cyan-700 px-3 py-2 text-sm font-medium text-white hover:bg-cyan-600 disabled:opacity-50 sm:col-span-2 lg:col-span-5">
              {saving ? "Saving…" : "Save rule"}
            </button>
          </form>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-800 text-xs text-slate-500">
              <tr>
                <th className="px-5 py-3">Name</th>
                <th className="px-5 py-3">OID pattern</th>
                <th className="px-5 py-3">Severity</th>
                <th className="px-5 py-3">Notify</th>
                <th className="px-5 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {rules.length ? rules.map(r => (
                <tr key={r.id} className="border-b border-slate-800/70">
                  <td className="px-5 py-3">{r.name}</td>
                  <td className="px-5 py-3 font-mono text-xs">{r.oidPattern}</td>
                  <td className="px-5 py-3">
                    <span className={`rounded-full px-2 py-1 text-xs ${severityBadge(r.severity)}`}>{r.severity}</span>
                  </td>
                  <td className="px-5 py-3 text-xs text-slate-400">{r.notifyEmail || r.notifyWebhookUrl || "—"}</td>
                  <td className="px-5 py-3 text-right">
                    <button onClick={() => deleteRule(r.id)} className="text-xs text-red-400 hover:text-red-300">Delete</button>
                  </td>
                </tr>
              )) : (
                <tr><td colSpan={5} className="px-5 py-6 text-center text-slate-500">No rules configured — traps default to "info" severity.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-xl border border-slate-800 bg-slate-900 overflow-hidden">
        <div className="border-b border-slate-800 px-5 py-4">
          <h2 className="text-sm font-semibold text-slate-200">Received traps</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-800 text-xs text-slate-500">
              <tr>
                <th className="px-3 py-3">Received</th>
                <th className="px-3 py-3">Source</th>
                <th className="px-3 py-3">Version</th>
                <th className="px-3 py-3">Severity</th>
                <th className="px-3 py-3">Trap OID</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">Loading…</td></tr>
              ) : traps.length ? traps.map(t => (
                <>
                  <tr key={t.id} className="cursor-pointer border-b border-slate-800/70 align-top hover:bg-slate-800/40" onClick={() => setExpanded(expanded === t.id ? null : t.id)}>
                    <td className="whitespace-nowrap px-3 py-3 text-xs text-slate-400">{new Date(t.receivedAt).toLocaleString()}</td>
                    <td className="px-3 py-3 font-mono text-xs">{t.sourceIp}</td>
                    <td className="px-3 py-3 text-xs text-slate-400">{t.snmpVersion}</td>
                    <td className="px-3 py-3">
                      <span className={`rounded-full px-2 py-1 text-xs ${severityBadge(t.severity)}`}>{t.severity}</span>
                    </td>
                    <td className="px-3 py-3 font-mono text-xs text-slate-200">{t.trapOid || "—"}</td>
                  </tr>
                  {expanded === t.id && (
                    <tr className="border-b border-slate-800/70 bg-slate-950/60">
                      <td colSpan={5} className="px-3 py-3">
                        <div className="space-y-1 font-mono text-xs text-slate-400">
                          {t.varbinds.length ? t.varbinds.map((v, i) => (
                            <div key={i}>{v.oid} = ({v.type}) {v.value}</div>
                          )) : <div>No varbinds recorded.</div>}
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              )) : (
                <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">No SNMP traps received yet.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}
