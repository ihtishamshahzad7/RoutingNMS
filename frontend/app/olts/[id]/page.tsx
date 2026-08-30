'use client'

import { useEffect, useMemo, useState } from 'react'

type ONU={id:string;name:string;serial:string;status:string;rxPowerDbm?:number;txPowerDbm?:number}
type PON={id:string;port:number;status:string;onus:ONU[]}
type Hierarchy={oltId:string;name:string;model?:string;pons:PON[];updatedAt:string}

type OLTDetailProps = { params: Promise<{ id: string }> }

export default function OLTDetail({params}: OLTDetailProps){
 const [id,setId]=useState('')
 const [data,setData]=useState<Hierarchy|null>(null),[error,setError]=useState(''),[filter,setFilter]=useState('all')
 useEffect(()=>{let live=true;const load=async()=>{try{const {id}=await params; if(live)setId(id);const r=await fetch(`/api/olts/${encodeURIComponent(id)}`,{cache:'no-store'});if(!r.ok)throw Error('OLT not found');const j=await r.json();if(live){setData(j);setError('')}}catch(e){if(live)setError(e instanceof Error?e.message:'Unable to load OLT')}};load();const t=setInterval(load,15000);return()=>{live=false;clearInterval(t)}},[params])
 const onus=useMemo(()=>data?.pons.flatMap(p=>p.onus.map(o=>({...o,pon:p.port}))).filter(o=>filter==='all'||o.status===filter)||[],[data,filter])
 if(error)return <main className="p-8"><div className="rounded-xl border p-6">{error}</div></main>
 if(!data)return <main className="p-8 text-muted-foreground">Loading OLT…</main>
 const total=data.pons.reduce((n,p)=>n+p.onus.length,0),online=data.pons.reduce((n,p)=>n+p.onus.filter(o=>o.status==='online').length,0)
 return <main className="min-h-screen p-6 md:p-8 space-y-6"><header><p className="text-xs font-semibold uppercase tracking-[.2em] text-muted-foreground">Fiber Operations</p><h1 className="text-3xl font-bold">{data.name}</h1><p className="text-muted-foreground">{data.model||'OLT'} · {data.oltId || id}</p></header><section className="grid grid-cols-2 md:grid-cols-4 gap-4"><Card label="PONs" value={data.pons.length}/><Card label="ONUs" value={total}/><Card label="Online" value={online}/><Card label="Offline" value={total-online}/></section><section className="grid md:grid-cols-3 gap-4">{data.pons.map(p=><div key={p.id} className="rounded-xl border p-5"><div className="flex justify-between"><b>PON {p.port}</b><span className="text-sm capitalize">{p.status}</span></div><div className="mt-3 text-2xl font-bold">{p.onus.length}</div><div className="text-xs text-muted-foreground">connected ONUs</div></div>)}</section><section className="rounded-xl border overflow-hidden"><div className="p-5 flex items-center justify-between"><h2 className="font-semibold">ONU optical health</h2><select className="rounded-lg border bg-background px-3 py-2 text-sm" value={filter} onChange={e=>setFilter(e.target.value)}><option value="all">All</option><option value="online">Online</option><option value="offline">Offline</option></select></div><div className="overflow-x-auto"><table className="w-full text-sm"><thead className="bg-muted/40"><tr><th className="text-left p-4">ONU</th><th className="text-left p-4">PON</th><th className="text-left p-4">Serial</th><th className="text-left p-4">Status</th><th className="text-left p-4">RX</th><th className="text-left p-4">TX</th></tr></thead><tbody>{onus.map(o=><tr key={o.id} className="border-t"><td className="p-4 font-medium">{o.name}</td><td className="p-4">{o.pon}</td><td className="p-4 font-mono text-xs">{o.serial}</td><td className="p-4 capitalize">{o.status}</td><td className="p-4">{o.rxPowerDbm??'—'} dBm</td><td className="p-4">{o.txPowerDbm??'—'} dBm</td></tr>)}</tbody></table></div></section></main>
}
function Card({label,value}:{label:string;value:number}){return <div className="rounded-xl border p-5"><div className="text-sm text-muted-foreground">{label}</div><div className="mt-2 text-3xl font-bold">{value}</div></div>}
