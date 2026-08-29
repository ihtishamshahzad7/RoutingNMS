'use client'

import { useEffect } from 'react'

type Incident={id:string;severity:string;title:string}
export function IncidentLive({onIncident}:{onIncident:(i:Incident)=>void}){
 useEffect(()=>{const source=new EventSource('/api/incidents/stream'); const handler=(e:MessageEvent)=>{try{onIncident(JSON.parse(e.data))}catch{}}; source.addEventListener('incident',handler); return()=>{source.removeEventListener('incident',handler);source.close()}},[onIncident]); return null
}
