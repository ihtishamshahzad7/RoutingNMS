'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

type Incident={id:string;severity:string;title:string;resourceId?:string}

export function CriticalAlertNotifications(){
 const [alerts,setAlerts]=useState<Incident[]>([]); const audio=useRef<AudioContext|null>(null)
 const notify=useCallback((i:Incident)=>{
  setAlerts(a=>a.some(x=>x.id===i.id)?a:[i,...a].slice(0,5))
  if(i.severity==='critical'){
   try{audio.current ??= new AudioContext(); const ctx=audio.current; const o=ctx.createOscillator(); const g=ctx.createGain(); o.frequency.value=880; g.gain.value=.06; o.connect(g);g.connect(ctx.destination);o.start();o.stop(ctx.currentTime+.35)}catch{}
   if(typeof Notification!=='undefined'&&Notification.permission==='granted') new Notification('Critical network incident',{body:`${i.title}${i.resourceId?` · ${i.resourceId}`:''}`})
  }
 },[])
 useEffect(()=>{if(typeof Notification!=='undefined'&&Notification.permission==='default') Notification.requestPermission(); const s=new EventSource('/api/incidents/stream'); const h=(e:MessageEvent)=>{try{notify(JSON.parse(e.data))}catch{}};s.addEventListener('incident',h);return()=>{s.removeEventListener('incident',h);s.close()}},[notify])
 return <div className="fixed right-4 top-4 z-50 w-[min(420px,calc(100vw-2rem))] space-y-2">{alerts.map(i=><div key={i.id} className="rounded-xl border bg-background p-4 shadow-lg"><div className="flex items-start justify-between gap-3"><div><div className="text-xs font-semibold uppercase tracking-wider">{i.severity}</div><div className="font-semibold">{i.title}</div>{i.resourceId&&<div className="text-xs text-muted-foreground">{i.resourceId}</div>}</div><button className="text-muted-foreground" onClick={()=>setAlerts(a=>a.filter(x=>x.id!==i.id))} aria-label="Dismiss">×</button></div></div>)}</div>
}
