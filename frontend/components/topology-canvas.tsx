'use client'

import { useMemo, useState } from 'react'

type Node={id:string;name:string;type:string;address?:string;health:number}
type Link={id:string;source:string;target:string;status:string;latencyMs?:number;packetLossPct?:number}

const pos=(i:number,total:number)=>({x:80+(i%4)*230,y:70+Math.floor(i/4)*150})
export function TopologyCanvas({nodes,links}:{nodes:Node[];links:Link[]}){
 const [selected,setSelected]=useState<string|null>(null)
 const locations=useMemo(()=>new Map(nodes.map((n,i)=>[n.id,pos(i,nodes.length)])),[nodes])
 return <div className="relative min-h-[520px] overflow-auto rounded-xl border bg-muted/10 p-4"><svg width={Math.max(1000,Math.ceil(nodes.length/4)*230)} height={Math.max(520,Math.ceil(nodes.length/4)*150)} className="absolute left-0 top-0">{links.map(l=>{const a=locations.get(l.source),b=locations.get(l.target);if(!a||!b)return null;return <g key={l.id}><line x1={a.x+80} y1={a.y+35} x2={b.x+80} y2={b.y+35} stroke="currentColor" strokeWidth={l.status==='down'?4:2} strokeDasharray={l.status==='degraded'?'7 5':undefined} opacity={.45}/><text x={(a.x+b.x)/2+80} y={(a.y+b.y)/2+30} textAnchor="middle" fontSize="11">{l.latencyMs??0}ms</text></g>})}</svg>{nodes.map((n,i)=>{const p=pos(i,nodes.length);return <button key={n.id} onClick={()=>setSelected(n.id)} style={{left:p.x,top:p.y}} className="absolute w-40 rounded-xl border bg-background p-3 text-left shadow-sm hover:ring-2"><div className="text-[10px] uppercase tracking-wider text-muted-foreground">{n.type}</div><div className="font-semibold truncate">{n.name}</div><div className="text-xs text-muted-foreground mt-1">Health {n.health}%</div></button>})}{selected&&<div className="absolute bottom-4 left-4 right-4 rounded-xl border bg-background p-4 shadow-lg">{(()=>{const n=nodes.find(x=>x.id===selected);return n&&<><div className="font-semibold">{n.name}</div><div className="text-xs text-muted-foreground">{n.type}{n.address?` · ${n.address}`:''} · Health {n.health}%</div></>})()}</div>}</div>
}
