'use client'

import { useEffect, useMemo, useState } from 'react'
import { TopologyCanvas } from '../../../components/topology-canvas'

type Node={id:string;name:string;type:string;address?:string;health:number}
type Link={id:string;source:string;target:string;status:string;latencyMs?:number;packetLossPct?:number}
type Graph={nodes:Node[];links:Link[];generatedAt:string}
type DiscoveryStatus={lastRun?:string;lastError?:string;links?:number;nodes?:number;running?:boolean;interval?:number}

export default function TopologyPage(){
 const [graph,setGraph]=useState<Graph>({nodes:[],links:[],generatedAt:''})
 const [status,setStatus]=useState<DiscoveryStatus>({})
 const [error,setError]=useState('')
 const [discovering,setDiscovering]=useState(false)

 const loadStatus=async()=>{try{const r=await fetch('/api/v1/topology/status',{cache:'no-store'});if(r.ok)setStatus(await r.json())}catch{}}

 const load=async()=>{try{const r=await fetch('/api/topology',{cache:'no-store'});if(!r.ok)throw new Error('Unable to load topology');setGraph(await r.json());setError('')}catch(e){setError(e instanceof Error?e.message:'Topology unavailable')}}
 useEffect(()=>{let live=true;const run=async()=>{await Promise.all([load(),loadStatus()])};run();const t=setInterval(()=>{if(live){load();loadStatus()}},10000);return()=>{live=false;clearInterval(t)}},[])

 const rediscover=async()=>{setDiscovering(true);try{await fetch('/api/v1/topology/discover',{method:'POST',cache:'no-store'});await Promise.all([load(),loadStatus()])}catch{}finally{setDiscovering(false)}}

 const stats=useMemo(()=>({up:graph.links.filter(x=>x.status==='up').length,down:graph.links.filter(x=>x.status==='down').length,degraded:graph.links.filter(x=>x.status==='degraded').length}),[graph.links])

 return <main className="min-h-screen p-6 md:p-8 space-y-6"><header><p className="text-xs font-semibold uppercase tracking-[.2em] text-muted-foreground">Network Operations Center</p><h1 className="text-3xl font-bold tracking-tight">Topology</h1><p className="text-muted-foreground">Discovered network relationships and link health.</p></header><section className="grid grid-cols-2 md:grid-cols-4 gap-4"><Card label="Devices" value={graph.nodes.length}/><Card label="Links" value={graph.links.length}/><Card label="Up" value={stats.up}/><Card label="Issues" value={stats.down+stats.degraded}/></section>{error&&<div className="rounded-xl border p-4 text-sm">{error}</div>}<section className="rounded-xl border p-5 space-y-4"><div className="flex items-center justify-between flex-wrap gap-3"><div className="space-y-1"><h2 className="font-semibold">Live topology</h2><div className="text-xs text-muted-foreground flex items-center gap-4 flex-wrap">{status.running?<span className="inline-flex items-center gap-1"><span className="h-2 w-2 animate-pulse rounded-full bg-emerald-500"/><span>Discovery running</span></span>:<span>Auto-refresh: 10s</span>}{status.lastRun&&<span>Last discovery {timeAgo(status.lastRun)}</span>}{typeof status.links==='number'&&<span>{status.links} links found</span>}</div></div><button onClick={rediscover} disabled={discovering||status.running} className="rounded-lg border border-input bg-background px-3 py-1.5 text-sm font-medium hover:bg-accent disabled:opacity-50">{discovering?'Discovering…':'Rediscover now'}</button></div><TopologyCanvas nodes={graph.nodes} links={graph.links}/></section><section className="rounded-xl border overflow-hidden"><div className="p-5 font-semibold">Discovered links</div><div className="overflow-x-auto"><table className="w-full text-sm"><thead className="bg-muted/40"><tr><th className="text-left p-4">Source</th><th className="text-left p-4">Target</th><th className="text-left p-4">Status</th><th className="text-left p-4">Latency</th><th className="text-left p-4">Loss</th></tr></thead><tbody>{graph.links.map(l=><tr key={l.id} className="border-t"><td className="p-4 font-mono text-xs">{l.source}</td><td className="p-4 font-mono text-xs">{l.target}</td><td className="p-4 capitalize">{l.status}</td><td className="p-4">{l.latencyMs??0} ms</td><td className="p-4">{l.packetLossPct??0}%</td></tr>)}</tbody></table></div></section></main>
}
function Card({label,value}:{label:string;value:number}){return <div className="rounded-xl border p-5"><div className="text-sm text-muted-foreground">{label}</div><div className="mt-2 text-3xl font-bold">{value}</div></div>}
function timeAgo(iso:string){try{const d=+new Date(iso);if(!d)return '';const s=Math.max(0,Math.floor((Date.now()-d)/1000));if(s<5)return 'just now';if(s<60)return `${s}s ago`;if(s<3600)return `${Math.floor(s/60)}m ago`;return `${Math.floor(s/3600)}h ago`}catch{return ''}}
