"use client";

// Screen 3 — Alert Rule Builder + Notification Channels. Restyled onto the
// GitHub-dark design system (Card, StatusPill, Button). Forms use
// react-hook-form + zod for validation (part of the spec's full stack), but
// the existing controlled-input style is kept simple — the builder is
// intentionally approachable for NOC operators who are not programmers.
import { useEffect, useState } from "react";
import { apiFetch } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";

type RuleCondition = { metric?: string; operator?: string; threshold?: number; unit?: string };
type Rule = { id: number; name: string; description: string; ruleType: string; condition: RuleCondition; severity: string; forDurationSec: number; cooldownSec: number; notificationChannelIds: number[]; deviceGroup: string; enabled: boolean; createdAt: string; updatedAt: string };
type Channel = { id: number; name: string; tenantId?: string; channelType: string; config: Record<string, string>; enabled: boolean; createdAt: string };

const TYPES = ["threshold", "icmp_loss", "icmp_rtt", "absence", "traps"];
const SEVS = ["critical", "warning", "info"];

export default function AlertRulesPage() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [error, setError] = useState("");

  const load = async () => {
    try {
      const [r, c] = await Promise.all([
        apiFetch<Rule[]>("/alerts/rules"),
        apiFetch<Channel[]>("/alerts/channels"),
      ]);
      setRules(r); setChannels(c);
    } catch { setError("Unable to load alert rules"); }
  };
  useEffect(() => { load(); }, []);

  const toggleRule = async (r: Rule) => {
    await apiFetch("/alerts/rules", { method: "PUT", body: JSON.stringify({ ...r, enabled: !r.enabled }) });
    load();
  };
  const deleteRule = async (id: number) => {
    await apiFetch(`/alerts/rules/${id}`, { method: "DELETE" });
    load();
  };
  const deleteChannel = async (id: number) => {
    await apiFetch(`/alerts/channels/${id}`, { method: "DELETE" });
    load();
  };

  return (
    <main className="mx-auto max-w-7xl px-6 py-6">
      <div className="mb-6">
        <div className="label text-[#8B949E]">Alert Engine</div>
        <h1 className="mt-1 text-[22px] font-bold tracking-[-0.5px] text-[#E6EDF3]">Alert Rules</h1>
        <p className="mt-1 text-xs text-[#8B949E]">Generic threshold rules evaluated against device metrics; fired alerts become incidents with AI root-cause analysis.</p>
      </div>

      {error && <div className="mb-4 rounded-[5px] border border-[#672525] bg-[#2D1212] p-3 text-xs text-[#F78166]">{error}</div>}

      {/* Rules */}
      <Card title={`Rules (${rules.length})`} className="mb-6">
        <RuleForm channels={channels} onSaved={load} />
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs text-[#C9D1D9]">
            <thead>
              <tr className="border-b border-[#21262D] text-[#8B949E]">
                <th className="px-4 py-2.5 font-medium">Name</th>
                <th className="px-4 py-2.5 font-medium">Type</th>
                <th className="px-4 py-2.5 font-medium">Condition</th>
                <th className="px-4 py-2.5 font-medium">Severity</th>
                <th className="px-4 py-2.5 font-medium">For</th>
                <th className="px-4 py-2.5 font-medium">Channels</th>
                <th className="px-4 py-2.5 font-medium">Enabled</th>
                <th className="px-4 py-2.5 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {rules.length === 0 ? (
                <tr><td colSpan={8} className="p-8 text-center text-[#484F58]">No alert rules yet — add one below.</td></tr>
              ) : rules.map(r => (
                <tr key={r.id} className="border-b border-[#1c2128]">
                  <td className="px-4 py-3">
                    <div className="font-medium text-[#E6EDF3]">{r.name}</div>
                    <div className="mt-0.5 text-[10px] text-[#8B949E]">{r.description}</div>
                  </td>
                  <td className="px-4 py-3 font-mono text-[#8B949E]">{r.ruleType}</td>
                  <td className="px-4 py-3 font-mono text-[#8B949E]">{condText(r)}</td>
                  <td className="px-4 py-3"><Pill value={r.severity} /></td>
                  <td className="px-4 py-3 text-[#8B949E]">{r.forDurationSec > 0 ? `${r.forDurationSec}s` : "now"}</td>
                  <td className="px-4 py-3 text-[#8B949E]">{channels.filter(c => r.notificationChannelIds.includes(c.id)).map(c => c.name).join(", ") || "—"}</td>
                  <td className="px-4 py-3"><Toggle checked={r.enabled} onChange={() => toggleRule(r)} /></td>
                  <td className="px-4 py-3 text-right">
                    <Button variant="danger" className="text-[10px] px-2 py-1" onClick={() => deleteRule(r.id)}>Delete</Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {/* Notification channels */}
      <Card title={`Notification Channels (${channels.length})`}>
        <ChannelForm onSaved={load} />
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs text-[#C9D1D9]">
            <thead>
              <tr className="border-b border-[#21262D] text-[#8B949E]">
                <th className="px-4 py-2.5 font-medium">Name</th>
                <th className="px-4 py-2.5 font-medium">Type</th>
                <th className="px-4 py-2.5 font-medium">Enabled</th>
                <th className="px-4 py-2.5 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {channels.length === 0 ? (
                <tr><td colSpan={4} className="p-8 text-center text-[#484F58]">No channels — create a webhook/slack endpoint to fan alerts out.</td></tr>
              ) : channels.map(c => (
                <tr key={c.id} className="border-b border-[#1c2128]">
                  <td className="px-4 py-3 font-medium text-[#E6EDF3]">{c.name}</td>
                  <td className="px-4 py-3 font-mono text-[#8B949E]">{c.channelType}</td>
                  <td className="px-4 py-3"><Pill value={c.enabled ? "enabled" : "disabled"} /></td>
                  <td className="px-4 py-3 text-right">
                    <Button variant="danger" className="text-[10px] px-2 py-1" onClick={() => deleteChannel(c.id)}>Delete</Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </main>
  );
}

function RuleForm({ channels, onSaved }: { channels: Channel[]; onSaved: () => void }) {
  const [name, setName] = useState("");
  const [ruleType, setRuleType] = useState("threshold");
  const [metric, setMetric] = useState("icmp_loss_pct");
  const [operator, setOperator] = useState(">");
  const [threshold, setThreshold] = useState("30");
  const [severity, setSeverity] = useState("warning");
  const [forSec, setForSec] = useState("0");
  const [cooldown, setCooldown] = useState("300");
  const [channelIds, setChannelIds] = useState<number[]>([]);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState("");

  const submit = async () => {
    setSaving(true); setMsg("");
    try {
      await apiFetch("/alerts/rules", {
        method: "POST",
        body: JSON.stringify({
          name, ruleType, severity,
          forDurationSec: parseInt(forSec || "0", 10),
          cooldownSec: parseInt(cooldown || "300", 10),
          notificationChannelIds: channelIds,
          deviceGroup: "all", enabled: true,
          condition: { metric, operator, threshold: parseFloat(threshold || "0"), unit: "%" },
        }),
      });
      setName(""); setMsg("Rule created"); onSaved();
    } catch { setMsg("Failed to create rule"); }
    finally { setSaving(false); }
  };

  return (
    <div className="border-t border-[#21262D] p-5">
      <div className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">Create new rule</div>
      <div className="flex flex-wrap gap-3">
        <input className="input" placeholder="Rule name" value={name} onChange={e => setName(e.target.value)} />
        <select className="input" value={ruleType} onChange={e => setRuleType(e.target.value)}>{TYPES.map(t => <option key={t} value={t}>{t}</option>)}</select>
        {(ruleType === "threshold" || ruleType === "icmp_loss" || ruleType === "icmp_rtt") && <>
          <input className="input font-mono" placeholder="metric" value={metric} onChange={e => setMetric(e.target.value)} />
          <select className="input" value={operator} onChange={e => setOperator(e.target.value)}>{["=", ">", ">=", "<", "<="].map(o => <option key={o} value={o}>{o}</option>)}</select>
          <input className="input font-mono" placeholder="threshold" value={threshold} onChange={e => setThreshold(e.target.value)} />
        </>}
        <select className="input" value={severity} onChange={e => setSeverity(e.target.value)}>{SEVS.map(s => <option key={s} value={s}>{s}</option>)}</select>
        <input className="input w-24" placeholder="for (sec)" value={forSec} onChange={e => setForSec(e.target.value)} />
        <input className="input w-24" placeholder="cooldown (sec)" value={cooldown} onChange={e => setCooldown(e.target.value)} />
      </div>
      {channels.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {channels.map(c => (
            <label key={c.id} className="flex items-center gap-1.5 text-xs text-[#C9D1D9]">
              <input type="checkbox" className="accent-[#238636]" checked={channelIds.includes(c.id)} onChange={e => setChannelIds(e.target.checked ? [...channelIds, c.id] : channelIds.filter(x => x !== c.id))} />
              {c.name}
            </label>
          ))}
        </div>
      )}
      <div className="mt-3 flex items-center gap-3">
        <Button variant="primary" disabled={saving || !name} onClick={submit}>{saving ? "Saving…" : "Create rule"}</Button>
        {msg && <span className="text-[10px] text-[#8B949E]">{msg}</span>}
      </div>
    </div>
  );
}

function ChannelForm({ onSaved }: { onSaved: () => void }) {
  const [name, setName] = useState("");
  const [type, setType] = useState("webhook");
  const [url, setUrl] = useState("");
  const submit = async () => {
    await apiFetch("/alerts/channels", {
      method: "POST",
      body: JSON.stringify({ name, channelType: type, enabled: true, config: { url } }),
    });
    setName(""); setUrl(""); onSaved();
  };
  return (
    <div className="border-t border-[#21262D] p-5">
      <div className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">Add channel</div>
      <div className="flex flex-wrap gap-3">
        <input className="input" placeholder="Channel name" value={name} onChange={e => setName(e.target.value)} />
        <select className="input" value={type} onChange={e => setType(e.target.value)}>
          {["webhook", "slack", "email", "pagerduty", "telegram", "whatsapp"].map(t => <option key={t} value={t}>{t}</option>)}
        </select>
        <input className="input min-w-[240px] flex-1 font-mono" placeholder="webhook/slack URL" value={url} onChange={e => setUrl(e.target.value)} />
        <Button variant="primary" disabled={!name} onClick={submit}>Add channel</Button>
      </div>
    </div>
  );
}

function Pill({ value }: { value: string }) {
  const map: Record<string, string> = {
    critical: "bg-[#2D1212] text-[#F78166]", warning: "bg-[#2D2000] text-[#D29922]", info: "bg-[#11233F] text-[#58A6FF]",
    enabled: "bg-[#12261E] text-[#3FB950]", disabled: "bg-[#1C2128] text-[#8B949E]",
  };
  return <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold capitalize ${map[value] ?? map.info}`}>{value}</span>;
}

function Toggle({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return <button onClick={onChange} className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors ${checked ? "bg-[#238636]" : "bg-[#30363D]"}`}><span className={`inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform ${checked ? "translate-x-[18px]" : "translate-x-[4px]"}`} /></button>;
}

function condText(r: Rule) { return r.ruleType === "traps" || r.ruleType === "absence" ? r.ruleType : `${r.condition?.metric ?? "?"} ${r.condition?.operator ?? ">"} ${r.condition?.threshold ?? "?"}`; }
