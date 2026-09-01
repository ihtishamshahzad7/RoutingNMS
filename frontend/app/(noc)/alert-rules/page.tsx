'use client'

import { useEffect, useState } from 'react'

type RuleCondition = { metric?: string; operator?: string; threshold?: number; unit?: string }
type Rule = { id: number; name: string; description: string; ruleType: string; condition: RuleCondition; severity: string; forDurationSec: number; cooldownSec: number; notificationChannelIds: number[]; deviceGroup: string; enabled: boolean; createdAt: string; updatedAt: string }
type Channel = { id: number; name: string; tenantId?: string; channelType: string; config: Record<string, string>; enabled: boolean; createdAt: string }

const TYPES = ['threshold', 'icmp_loss', 'icmp_rtt', 'absence', 'traps']
const SEVS = ['critical', 'warning', 'info']

export default function AlertRulesPage() {
  const [rules, setRules] = useState<Rule[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [error, setError] = useState('')

  const load = async () => {
    try {
      const [r, c] = await Promise.all([
        fetch('/api/v1/alerts/rules', { cache: 'no-store' }),
        fetch('/api/v1/alerts/channels', { cache: 'no-store' }),
      ])
      if (r.ok) setRules(await r.json())
      if (c.ok) setChannels(await c.json())
    } catch { setError('Unable to load alert rules') }
  }
  useEffect(() => { load() }, [])

  return <main className="min-h-screen p-6 md:p-8 space-y-6">
    <header><p className="text-xs font-semibold uppercase tracking-[.2em] text-muted-foreground">Network Operations Center</p><h1 className="text-3xl font-bold tracking-tight">Alert Rules</h1><p className="text-muted-foreground">Generic threshold rules evaluated against device metrics; fired alerts become incidents with AI root-cause analysis.</p></header>
    {error && <div className="rounded-xl border p-4 text-sm">{error}</div>}

    <section className="rounded-xl border overflow-hidden">
      <div className="p-5 flex items-center justify-between"><h2 className="font-semibold">Rules ({rules.length})</h2></div>
      <RuleForm channels={channels} onSaved={load} />
      <div className="overflow-x-auto"><table className="w-full text-sm"><thead className="bg-muted/40"><tr><th className="text-left p-4">Name</th><th className="text-left p-4">Type</th><th className="text-left p-4">Condition</th><th className="text-left p-4">Severity</th><th className="text-left p-4">For</th><th className="text-left p-4">Channels</th><th className="text-left p-4">Enabled</th><th className="p-4 text-right">Actions</th></tr></thead><tbody>{rules.length === 0 ? <tr><td colSpan={8} className="p-8 text-center text-muted-foreground">No alert rules yet — add one below.</td></tr> : rules.map(r => <tr key={r.id} className="border-t"><td className="p-4"><div className="font-medium">{r.name}</div><div className="text-xs text-muted-foreground">{r.description}</div></td><td className="p-4 font-mono text-xs">{r.ruleType}</td><td className="p-4 font-mono text-xs">{condText(r)}</td><td className="p-4"><Badge value={r.severity} /></td><td className="p-4 text-xs">{r.forDurationSec > 0 ? `${r.forDurationSec}s` : 'now'}</td><td className="p-4 text-xs">{channels.filter(c => r.notificationChannelIds.includes(c.id)).map(c => c.name).join(', ') || '—'}</td><td className="p-4"><Toggle checked={r.enabled} onChange={async () => { await fetch(`/api/v1/alerts/rules`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...r, enabled: !r.enabled }) }); load() }} /></td><td className="p-4 text-right space-x-2"><button className="rounded-md border px-3 py-1.5" onClick={async () => { await fetch(`/api/v1/alerts/rules/${r.id}`, { method: 'DELETE' }); load() }}>Delete</button></td></tr>)}</tbody></table></div>
    </section>

    <section className="rounded-xl border overflow-hidden">
      <div className="p-5"><h2 className="font-semibold">Notification channels ({channels.length})</h2></div>
      <ChannelForm onSaved={load} />
      <div className="overflow-x-auto"><table className="w-full text-sm"><thead className="bg-muted/40"><tr><th className="text-left p-4">Name</th><th className="text-left p-4">Type</th><th className="text-left p-4">Enabled</th><th className="p-4 text-right">Actions</th></tr></thead><tbody>{channels.length === 0 ? <tr><td colSpan={4} className="p-8 text-center text-muted-foreground">No channels — create a webhook/slack endpoint to fan alerts out.</td></tr> : channels.map(c => <tr key={c.id} className="border-t"><td className="p-4 font-medium">{c.name}</td><td className="p-4 font-mono text-xs">{c.channelType}</td><td className="p-4"><Badge value={c.enabled ? 'enabled' : 'disabled'} /></td><td className="p-4 text-right"><button className="rounded-md border px-3 py-1.5" onClick={async () => { await fetch(`/api/v1/alerts/channels/${c.id}`, { method: 'DELETE' }); load() }}>Delete</button></td></tr>)}</tbody></table></div>
    </section>
  </main>
}

function RuleForm({ channels, onSaved }: { channels: Channel[]; onSaved: () => void }) {
  const [name, setName] = useState('')
  const [ruleType, setRuleType] = useState('threshold')
  const [metric, setMetric] = useState('icmp_loss_pct')
  const [operator, setOperator] = useState('>')
  const [threshold, setThreshold] = useState('30')
  const [severity, setSeverity] = useState('warning')
  const [forSec, setForSec] = useState('0')
  const [cooldown, setCooldown] = useState('300')
  const [channelIds, setChannelIds] = useState<number[]>([])
  const [saving, setSaving] = useState(false)
  const [msg, setMsg] = useState('')

  const submit = async () => {
    setSaving(true); setMsg('')
    try {
      const res = await fetch('/api/v1/alerts/rules', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name, ruleType, severity,
          forDurationSec: parseInt(forSec || '0', 10),
          cooldownSec: parseInt(cooldown || '300', 10),
          notificationChannelIds: channelIds,
          deviceGroup: 'all', enabled: true,
          condition: { metric, operator, threshold: parseFloat(threshold || '0'), unit: '%' },
        }),
      })
      if (res.ok) { setName(''); setMsg('Rule created'); onSaved() } else setMsg('Failed to create rule')
    } catch { setMsg('Failed to create rule') } finally { setSaving(false) }
  }

  return <div className="border-t p-5 space-y-3">
    <div className="flex flex-wrap gap-3">
      <input className="rounded-lg border px-3 py-2 text-sm" placeholder="Rule name" value={name} onChange={e => setName(e.target.value)} />
      <select className="rounded-lg border px-3 py-2 text-sm" value={ruleType} onChange={e => setRuleType(e.target.value)}>{TYPES.map(t => <option key={t} value={t}>{t}</option>)}</select>
      {(ruleType === 'threshold' || ruleType === 'icmp_loss' || ruleType === 'icmp_rtt') && <>
        <input className="rounded-lg border px-3 py-2 text-sm font-mono" placeholder="metric" value={metric} onChange={e => setMetric(e.target.value)} />
        <select className="rounded-lg border px-3 py-2 text-sm" value={operator} onChange={e => setOperator(e.target.value)}>{['>', '>=', '<', '<='].map(o => <option key={o} value={o}>{o}</option>)}</select>
        <input className="rounded-lg border px-3 py-2 text-sm font-mono" placeholder="threshold" value={threshold} onChange={e => setThreshold(e.target.value)} />
      </>}
      <select className="rounded-lg border px-3 py-2 text-sm" value={severity} onChange={e => setSeverity(e.target.value)}>{SEVS.map(s => <option key={s} value={s}>{s}</option>)}</select>
      <input className="rounded-lg border px-3 py-2 text-sm" placeholder="for (sec)" value={forSec} onChange={e => setForSec(e.target.value)} />
      <input className="rounded-lg border px-3 py-2 text-sm" placeholder="cooldown (sec)" value={cooldown} onChange={e => setCooldown(e.target.value)} />
    </div>
    {channels.length > 0 && <div className="flex flex-wrap gap-2">{channels.map(c => <label key={c.id} className="flex items-center gap-1.5 text-sm"><input type="checkbox" checked={channelIds.includes(c.id)} onChange={e => setChannelIds(e.target.checked ? [...channelIds, c.id] : channelIds.filter(x => x !== c.id))} />{c.name}</label>)}</div>}
    <button disabled={saving || !name} onClick={submit} className="rounded-lg bg-primary text-primary-foreground px-4 py-2 text-sm font-medium disabled:opacity-50">{saving ? 'Saving…' : 'Create rule'}</button>
    {msg && <span className="text-xs text-muted-foreground">{msg}</span>}
  </div>
}

function ChannelForm({ onSaved }: { onSaved: () => void }) {
  const [name, setName] = useState('')
  const [type, setType] = useState('webhook')
  const [url, setUrl] = useState('')
  const submit = async () => {
    const res = await fetch('/api/v1/alerts/channels', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, channelType: type, enabled: true, config: { url } }),
    })
    if (res.ok) { setName(''); setUrl(''); onSaved() }
  }
  return <div className="border-t p-5 flex flex-wrap gap-3">
    <input className="rounded-lg border px-3 py-2 text-sm" placeholder="Channel name" value={name} onChange={e => setName(e.target.value)} />
    <select className="rounded-lg border px-3 py-2 text-sm" value={type} onChange={e => setType(e.target.value)}>{['webhook', 'slack', 'email', 'pagerduty', 'telegram', 'whatsapp'].map(t => <option key={t} value={t}>{t}</option>)}</select>
    <input className="rounded-lg border px-3 py-2 text-sm font-mono flex-1 min-w-[240px]" placeholder="webhook/slack URL" value={url} onChange={e => setUrl(e.target.value)} />
    <button disabled={!name} onClick={submit} className="rounded-lg bg-primary text-primary-foreground px-4 py-2 text-sm font-medium disabled:opacity-50">Add channel</button>
  </div>
}

function Badge({ value }: { value: string }) { return <span className="inline-flex rounded-full border px-2.5 py-1 text-xs font-medium capitalize">{value}</span> }
function Toggle({ checked, onChange }: { checked: boolean; onChange: () => void }) { return <button onClick={onChange} className={`relative h-5 w-9 rounded-full transition-colors ${checked ? 'bg-emerald-500' : 'bg-muted'}`}><span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-all ${checked ? 'left-4.5' : 'left-0.5'}`} /></button> }
function condText(r: Rule) { return r.ruleType === 'traps' || r.ruleType === 'absence' ? r.ruleType : `${r.condition?.metric ?? '?'} ${r.condition?.operator ?? '>'} ${r.condition?.threshold ?? '?'}` }