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
type Preset = { id: string; name: string; description: string; ruleType: string; metric: string; operator: string; threshold: number; unit?: string; severity: string };

const TYPES = ["threshold", "icmp_loss", "icmp_rtt", "absence", "traps"];
const SEVS = ["critical", "warning", "info"];

export default function AlertRulesPage() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [error, setError] = useState("");

  const load = async () => {
    try {
      const [r, c, p] = await Promise.all([
        apiFetch<Rule[]>("/alerts/rules"),
        apiFetch<Channel[]>("/alerts/channels"),
        apiFetch<Preset[]>("/alerts/presets").catch(() => []),
      ]);
      setRules(r); setChannels(c); setPresets(p);
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
        <RuleForm channels={channels} presets={presets} onSaved={load} />
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

function RuleForm({ channels, presets, onSaved }: { channels: Channel[]; presets: Preset[]; onSaved: () => void }) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
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

  // Quick presets: selecting one pre-fills + locks the condition fields
  // below (metric/operator/threshold/type/severity) so a user doesn't have
  // to already know the internal metric name. "Custom" (the default)
  // reverts to the fully manual fields, unchanged from before presets
  // existed -- nothing about the custom path is altered by this feature.
  const [presetId, setPresetId] = useState("custom");
  const applyPreset = (id: string) => {
    setPresetId(id);
    if (id === "custom") return;
    const p = presets.find(x => x.id === id);
    if (!p) return;
    setRuleType(p.ruleType);
    setMetric(p.metric);
    setOperator(p.operator);
    setThreshold(String(p.threshold));
    setSeverity(p.severity);
    if (!name) setName(p.name);
    if (!description) setDescription(p.description);
  };
  const locked = presetId !== "custom";

  const submit = async () => {
    setSaving(true); setMsg("");
    try {
      await apiFetch("/alerts/rules", {
        method: "POST",
        body: JSON.stringify({
          name, description, ruleType, severity,
          forDurationSec: parseInt(forSec || "0", 10),
          cooldownSec: parseInt(cooldown || "300", 10),
          notificationChannelIds: channelIds,
          deviceGroup: "all", enabled: true,
          condition: { metric, operator, threshold: parseFloat(threshold || "0"), unit: "%" },
        }),
      });
      setName(""); setDescription(""); setPresetId("custom"); setMsg("Rule created"); onSaved();
    } catch { setMsg("Failed to create rule"); }
    finally { setSaving(false); }
  };

  return (
    <div className="border-t border-[#21262D] p-5">
      <div className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">Create new rule</div>

      {presets.length > 0 && (
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="text-[10px] uppercase tracking-wider text-[#8B949E]">Quick preset</span>
          <select className="input" value={presetId} onChange={e => applyPreset(e.target.value)}>
            <option value="custom">Custom (advanced)</option>
            {presets.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
          {presetId !== "custom" && (
            <>
              <span className="text-[10px] text-[#8B949E]">{presets.find(p => p.id === presetId)?.description}</span>
              <Button variant="secondary" className="text-[10px] px-2 py-1" onClick={() => setPresetId("custom")}>Unlock / edit manually</Button>
            </>
          )}
        </div>
      )}

      <div className="flex flex-wrap gap-3">
        <input className="input" placeholder="Rule name" value={name} onChange={e => setName(e.target.value)} />
        <input className="input min-w-[220px] flex-1" placeholder="Description (optional)" value={description} onChange={e => setDescription(e.target.value)} />
        <select className="input" value={ruleType} disabled={locked} onChange={e => setRuleType(e.target.value)}>{TYPES.map(t => <option key={t} value={t}>{t}</option>)}</select>
        {(ruleType === "threshold" || ruleType === "icmp_loss" || ruleType === "icmp_rtt") && <>
          <input className="input font-mono" placeholder="metric" value={metric} disabled={locked} onChange={e => setMetric(e.target.value)} />
          <select className="input" value={operator} disabled={locked} onChange={e => setOperator(e.target.value)}>{["=", ">", ">=", "<", "<="].map(o => <option key={o} value={o}>{o}</option>)}</select>
          <input className="input font-mono" placeholder="threshold" value={threshold} disabled={locked} onChange={e => setThreshold(e.target.value)} />
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

// Per-channel-type config fields -- each entry is [fieldKey, placeholder].
// Matches exactly what backend/internal/alerts/notify.go reads out of
// `config` for that channel type.
const CHANNEL_FIELDS: Record<string, [string, string][]> = {
  webhook: [["url", "Webhook URL"]],
  slack: [["webhook_url", "Slack incoming webhook URL"]],
  email: [
    ["smtp_host", "SMTP host (e.g. smtp.gmail.com)"],
    ["smtp_port", "SMTP port (default 587)"],
    ["smtp_username", "SMTP username (optional)"],
    ["smtp_password", "SMTP password (optional)"],
    ["from", "From address"],
    ["to", "To address(es), comma-separated"],
  ],
  telegram: [
    ["bot_token", "Bot token (from @BotFather)"],
    ["chat_id", "Chat ID"],
  ],
  pagerduty: [["routing_key", "Events API v2 routing (integration) key"]],
  whatsapp: [
    ["account_sid", "Twilio Account SID"],
    ["auth_token", "Twilio Auth Token"],
    ["from", "From (e.g. whatsapp:+14155238886)"],
    ["to", "To (e.g. whatsapp:+15551234567)"],
  ],
  discord: [["webhook_url", "Discord webhook URL (Server Settings → Integrations → Webhooks)"]],
  teams: [["webhook_url", "Teams incoming webhook URL"]],
  ntfy: [
    ["server_url", "ntfy server/topic URL (e.g. https://ntfy.sh)"],
    ["topic", "Topic"],
    ["priority", "Priority 1-5 (optional, default 4)"],
    ["auth_method", "Auth method: usernamePassword / accessToken (optional)"],
    ["username", "Username (optional)"],
    ["password", "Password (optional)"],
    ["access_token", "Access token (optional)"],
  ],
  gotify: [
    ["server_url", "Gotify server URL"],
    ["app_token", "Application token"],
    ["priority", "Priority (optional, default 8)"],
  ],
  pushover: [
    ["user_key", "User key"],
    ["app_token", "API token/key"],
    ["sound", "Sound (optional)"],
    ["priority", "Priority (optional)"],
    ["title", "Title (optional)"],
    ["device", "Device (optional)"],
    ["ttl", "TTL seconds (optional)"],
  ],
  matrix: [
    ["homeserver_url", "Matrix homeserver URL (e.g. https://matrix.org)"],
    ["access_token", "Access token"],
    ["room_id", "Room ID (e.g. !room:matrix.org)"],
  ],
  google_chat: [["webhook_url", "Google Chat incoming webhook URL"]],
  mattermost: [
    ["webhook_url", "Mattermost incoming webhook URL"],
    ["username", "Username (optional, default RoutingNMS)"],
    ["channel", "Channel (optional)"],
    ["icon_emoji", "Icon emoji (optional)"],
    ["icon_url", "Icon URL (optional)"],
  ],
  opsgenie: [
    ["api_key", "Opsgenie API key"],
    ["region", "Region: us / eu (optional, default us)"],
    ["priority", "Priority 1-5 (optional, default 3)"],
  ],
  signal: [
    ["signal_url", "signal-cli-rest-api URL"],
    ["number", "Sender number"],
    ["recipients", "Recipients, comma-separated"],
  ],
  bark: [
    ["endpoint", "Bark server endpoint (e.g. https://api.day.app/XXXXXXXX)"],
    ["group", "Group (optional, default RoutingNMS)"],
    ["sound", "Sound (optional, default telegraph)"],
  ],
  line: [
    ["channel_access_token", "Channel access token"],
    ["user_id", "User ID"],
  ],
  alerta: [
    ["api_endpoint", "Alerta API endpoint"],
    ["api_key", "API key"],
    ["environment", "Environment (optional)"],
    ["alert_state", "Alert state (optional, default critical)"],
    ["recover_state", "Recover state (optional, default cleared)"],
  ],
  squadcast: [["webhook_url", "Squadcast webhook URL"]],
  pagertree: [
    ["integration_url", "PagerTree integration URL"],
    ["urgency", "Urgency (optional)"],
  ],
  splunk: [
    ["rest_url", "Splunk On-Call REST endpoint URL"],
    ["routing_key", "Routing key"],
    ["severity", "Severity (optional, default critical)"],
    ["auto_resolve", "Auto-resolve message type (optional; blank/0 skips resolve notifications)"],
  ],
  stackfield: [["webhook_url", "Stackfield webhook URL"]],
  wecom: [["bot_key", "WeCom bot key"]],
  feishu: [["webhook_url", "Feishu (Lark) webhook URL"]],
  home_assistant: [
    ["base_url", "Home Assistant base URL"],
    ["long_lived_token", "Long-lived access token"],
    ["notification_service", "Notification service (optional, default notify)"],
  ],
};

function ChannelForm({ onSaved }: { onSaved: () => void }) {
  const [name, setName] = useState("");
  const [type, setType] = useState("webhook");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [msg, setMsg] = useState("");
  const changeType = (t: string) => { setType(t); setFields({}); };
  const submit = async () => {
    setMsg("");
    try {
      await apiFetch("/alerts/channels", {
        method: "POST",
        body: JSON.stringify({ name, channelType: type, enabled: true, config: fields }),
      });
      setName(""); setFields({}); onSaved();
    } catch {
      setMsg("Failed to save channel.");
    }
  };
  const required = CHANNEL_FIELDS[type] ?? [];
  return (
    <div className="border-t border-[#21262D] p-5">
      <div className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-[#8B949E]">Add channel</div>
      <div className="flex flex-wrap gap-3">
        <input className="input" placeholder="Channel name" value={name} onChange={e => setName(e.target.value)} />
        <select className="input" value={type} onChange={e => changeType(e.target.value)}>
          {["webhook", "slack", "email", "pagerduty", "telegram", "whatsapp", "discord", "teams", "ntfy", "gotify", "pushover", "matrix", "google_chat", "mattermost", "opsgenie", "signal", "bark", "line", "alerta", "squadcast", "pagertree", "splunk", "stackfield", "wecom", "feishu", "home_assistant"].map(t => <option key={t} value={t}>{t}</option>)}
        </select>
        {required.map(([key, placeholder]) => (
          <input
            key={key}
            type={key.toLowerCase().includes("password") || key.toLowerCase().includes("token") ? "password" : "text"}
            className="input min-w-[220px] flex-1 font-mono"
            placeholder={placeholder}
            value={fields[key] ?? ""}
            onChange={e => setFields(f => ({ ...f, [key]: e.target.value }))}
          />
        ))}
        <Button variant="primary" disabled={!name} onClick={submit}>Add channel</Button>
      </div>
      {msg && <div className="mt-2 text-[10px] text-[#F78166]">{msg}</div>}
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
