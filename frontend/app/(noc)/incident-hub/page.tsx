'use client'

import { useEffect, useState } from 'react'

type AiIncident = {
  id: number; incidentRef: string; status: string; severity: string; title: string; source: string; resourceId: string; deviceId?: number; triggeredAt: string;
  rootCause?: string; confidencePct?: number; affectedServices?: string[]; recommendedActions?: string[]; estimatedImpact?: string; timeline?: { t: string; event: string; detail?: string }[]; rcaCompletedAt?: string;
}

export default function IncidentHubPage() {
  const [items, setItems] = useState<AiIncident[]>([])
  const [selected, setSelected] = useState<AiIncident | null>(null)
  const [status, setStatus] = useState('')
  const [error, setError] = useState('')

  const load = async () => {
    try {
      const q = status ? `?status=${status}` : ''
      const res = await fetch(`/api/v1/alerts/incidents${q}`, { cache: 'no-store' })
      if (!res.ok) throw new Error('Unable to load incidents')
      setItems(await res.json()); setError('')
    } catch (e) { setError(e instanceof Error ? e.message : 'Incident hub unavailable') }
  }
  useEffect(() => { load() }, [status])

  const open = async (id: number) => {
    const res = await fetch(`/api/v1/alerts/incidents/${id}`, { cache: 'no-store' })
    if (res.ok) setSelected(await res.json())
  }
  const act = async (id: number, action: 'acknowledge' | 'resolve') => {
    await fetch(`/api/v1/alerts/incidents/${id}/${action}`, { method: 'POST' })
    load(); if (selected) open(selected.id)
  }

  const counts = {
    critical: items.filter(i => i.severity === 'critical' && i.status !== 'resolved').length,
    open: items.filter(i => i.status === 'open' || i.status === 'analyzing').length,
    resolved: items.filter(i => i.status === 'resolved').length,
  }

  return <main className="min-h-screen p-6 md:p-8 space-y-6">
    <header><p className="text-xs font-semibold uppercase tracking-[.2em] text-muted-foreground">Network Operations Center</p><h1 className="text-3xl font-bold tracking-tight">Incident Hub</h1><p className="text-muted-foreground">Durable incident history with AI root-cause analysis.</p></header>
    {error && <div className="rounded-xl border p-4 text-sm">{error}</div>}
    <section className="grid grid-cols-1 sm:grid-cols-3 gap-4"><Card label="Critical" value={counts.critical} /><Card label="Open" value={counts.open} /><Card label="Resolved" value={counts.resolved} /></section>
    <section className="flex items-center gap-3">
      <select className="rounded-lg border bg-background px-3 py-2 text-sm" value={status} onChange={e => setStatus(e.target.value)}>
        <option value="">All statuses</option><option value="open">Open</option><option value="acknowledged">Acknowledged</option><option value="resolved">Resolved</option><option value="analyzing">Analyzing</option>
      </select>
      <button onClick={load} className="rounded-lg border px-3 py-2 text-sm">Refresh</button>
    </section>
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <section className="rounded-xl border overflow-hidden">
        <div className="p-5 font-semibold">Incidents ({items.length})</div>
        <div className="overflow-x-auto"><table className="w-full text-sm"><thead className="bg-muted/40"><tr><th className="text-left p-4">Incident</th><th className="text-left p-4">Severity</th><th className="text-left p-4">Status</th><th className="text-left p-4">RCA</th><th className="text-left p-4">Triggered</th></tr></thead><tbody>{items.length === 0 ? <tr><td colSpan={5} className="p-8 text-center text-muted-foreground">No incidents yet.</td></tr> : items.map(i => <tr key={i.id} className="border-t cursor-pointer hover:bg-muted/40" onClick={() => open(i.id)}><td className="p-4"><div className="font-medium">{i.title}</div><div className="text-xs text-muted-foreground">{i.source} · {i.resourceId}</div></td><td className="p-4"><Badge value={i.severity} /></td><td className="p-4"><Badge value={i.status} /></td><td className="p-4">{i.confidencePct != null ? <span className="text-xs">{i.confidencePct}%</span> : <span className="text-xs text-muted-foreground">pending</span>}</td><td className="p-4 text-muted-foreground">{new Date(i.triggeredAt).toLocaleString()}</td></tr>)}</tbody></table></div>
      </section>
      <RcaPanel incident={selected} onAction={act} />
    </div>
  </main>
}

function RcaPanel({ incident, onAction }: { incident: AiIncident | null; onAction: (id: number, a: 'acknowledge' | 'resolve') => void }) {
  if (!incident) return <section className="rounded-xl border p-8 text-center text-muted-foreground">Select an incident to view its root-cause analysis.</section>
  return <section className="rounded-xl border p-5 space-y-4">
    <div className="flex items-start justify-between gap-3 flex-wrap">
      <div><h2 className="text-lg font-semibold">{incident.title}</h2><p className="text-xs text-muted-foreground">#{incident.id} · {incident.source} · {incident.resourceId} · triggered {new Date(incident.triggeredAt).toLocaleString()}</p></div>
      <div className="space-x-2">{incident.status !== 'resolved' && <button className="rounded-md border px-3 py-1.5 text-sm" onClick={() => onAction(incident.id, 'acknowledge')}>Acknowledge</button>}{incident.status !== 'resolved' && <button className="rounded-md bg-primary text-primary-foreground px-3 py-1.5 text-sm" onClick={() => onAction(incident.id, 'resolve')}>Resolve</button>}</div>
    </div>
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <Field label="Root cause">{incident.rootCause || 'Pending analysis…'}</Field>
      <Field label="Confidence">{incident.confidencePct != null ? `${incident.confidencePct}%` : '—'}</Field>
    </div>
    <Field label="Estimated impact">{incident.estimatedImpact || '—'}</Field>
    <Field label="Affected services">{incident.affectedServices?.join(', ') || '—'}</Field>
    <div><div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">Recommended actions</div><ul className="space-y-1.5">{incident.recommendedActions?.length ? incident.recommendedActions.map((a, i) => <li key={i} className="text-sm rounded-lg border p-2.5">{a}</li>) : <li className="text-sm text-muted-foreground">None</li>}</ul></div>
    {incident.timeline?.length ? <div><div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">Timeline</div><ul className="space-y-1 text-sm">{incident.timeline.map((t, i) => <li key={i} className="flex gap-2"><span className="text-muted-foreground font-mono text-xs pt-0.5">{t.t?.slice(11, 19)}</span><span>{t.event}{t.detail ? ` — ${t.detail}` : ''}</span></li>)}</ul></div> : null}
  </section>
}

function Card({ label, value }: { label: string; value: number }) { return <div className="rounded-xl border p-5"><div className="text-sm text-muted-foreground">{label}</div><div className="mt-2 text-3xl font-bold">{value}</div></div> }
function Badge({ value }: { value: string }) { return <span className="inline-flex rounded-full border px-2.5 py-1 text-xs font-medium capitalize">{value}</span> }
function Field({ label, children }: { label: string; children: React.ReactNode }) { return <div><div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1">{label}</div><div className="text-sm">{children}</div></div> }