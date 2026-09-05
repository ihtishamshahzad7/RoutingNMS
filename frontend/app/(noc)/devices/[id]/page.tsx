"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";
import { ApiError, apiFetch } from "../../../../lib/api";
import { MetricChart } from "../../../../components/metric-chart";

type Device={id:string;name:string;address:string;deviceType:string;vendor?:string;serialNumber?:string;enabled:boolean;snmpEnabled:boolean;snmpVersion:string;snmpPort:number;snmpConfigured:boolean;provisioningTemplateId?:number|null;lastProvisionedAt?:string;httpCheckEnabled:boolean;httpUrl?:string;httpExpectedStatus:number;httpKeyword?:string;httpTimeoutMs:number;icmpEnabled:boolean;icmpIntervalSeconds:number;icmpPacketSize:number;icmpCount:number;icmpRetries:number;dnsEnabled:boolean;dnsHostname?:string;dnsRecordType:string;dnsResolverServer?:string;dnsExpectedAnswer?:string;dnsIntervalSeconds:number;pushEnabled:boolean;pushToken?:string;pushIntervalSeconds:number;pushGracePeriodSeconds:number;pushLastSeenAt?:string;pushLastStatus?:string;pushLastMessage?:string;sshEnabled:boolean;sshPort:number;sshBannerKeyword?:string;sshTimeoutMs:number;sshIntervalSeconds:number;telnetEnabled:boolean;telnetPort:number;telnetBannerKeyword?:string;telnetTimeoutMs:number;telnetIntervalSeconds:number};
type DNSLive={live:{resolved:boolean;answers?:string[];latencyMs:number;expectedMatch?:boolean|null;error?:string}};
type ReachLive={live:{reachable:boolean;banner?:string;latencyMs:number;bannerMatched?:boolean|null;error?:string}};
type ProvTemplate={id:number;name:string;scriptBody:string};
type Preview={renderedScript:string;password:string;fetchCommand:string};
type Interface={id:number;deviceId:string;ifIndex:number;name:string;description:string;adminUp:boolean;operUp:boolean;inOctets:number;outOctets:number;inErrors:number;outErrors:number;lastDiscoveredAt?:string};
type PingResult={id:number;deviceId:string;probedAt:string;rttMs?:number|null;jitterMs?:number|null;lossPct:number;ttl?:number|null;isReachable:boolean};
type PingLive={live?:{address?:string;reachable:boolean;rttMs?:number;jitterMs?:number;lossPct?:number;ttl?:number;error?:string};history:PingResult[]};
type Tag={id:number;name:string;color:string};
type TraceHop={number:number;address?:string;hostname?:string;rttMs?:number|null;timedOut:boolean};
type TraceResult={address:string;hops:TraceHop[];ranAt:string;error?:string};

const ORG="tenant-1";
const card="rounded-2xl border border-slate-800 bg-slate-900 p-5";

export default function DeviceDetailsPage(){
 const params=useParams<{id:string}>(); const id=params.id;
 const [device,setDevice]=useState<Device|null>(null),[interfaces,setInterfaces]=useState<Interface[]>([]),[loading,setLoading]=useState(true),[discovering,setDiscovering]=useState(false),[message,setMessage]=useState("");
 const [templates,setTemplates]=useState<ProvTemplate[]>([]),[preview,setPreview]=useState<Preview|null>(null),[provSaving,setProvSaving]=useState(false),[provLoading,setProvLoading]=useState(false),[provError,setProvError]=useState("");
 const [pingState,setPingState]=useState<{live:PingLive|null;probing:boolean;pingError:string}>({live:null,probing:false,pingError:""});
 async function loadPing(){try{const live=await apiFetch<PingLive>(`/ping/${id}/live`);setPingState(s=>({...s,live,pingError:""}))}catch(e){setPingState(s=>({...s,live:null,pingError:e instanceof ApiError?e.message:"Unable to load ping status."}))}}
 async function pingNow(){setPingState(s=>({...s,probing:true,pingError:""}));try{await apiFetch(`/ping/${id}/probe`,{method:"POST"});await loadPing()}catch(e){setPingState(s=>({...s,probing:false,pingError:e instanceof ApiError?e.message:"Ping probe failed."}))}finally{setPingState(s=>({...s,probing:false}))}}
 async function load(){setLoading(true);try{const devices=await apiFetch<Device[]>(`/devices?organizationId=${ORG}`);const d=devices.find(x=>x.id===id);if(!d)throw new Error("Device not found");setDevice(d);setInterfaces(await apiFetch<Interface[]>(`/devices/${id}/interfaces`))}catch(e){setMessage(e instanceof ApiError?e.message:e instanceof Error?e.message:"Unable to load device")}finally{setLoading(false)}}
 useEffect(()=>{load()},[id]);
 useEffect(()=>{apiFetch<ProvTemplate[]>("/provisioning/templates").then(setTemplates).catch(()=>{})},[]);
 useEffect(()=>{loadPing();const t=setInterval(loadPing,10000);return()=>clearInterval(t)},[id]);
 async function assignTemplate(templateId:string){if(!device)return;setProvSaving(true);setProvError("");try{const updated=await apiFetch<Device>(`/devices/${device.id}/provisioning`,{method:"PUT",body:JSON.stringify({templateId:templateId?Number(templateId):null})});setDevice(updated);setPreview(null)}catch(e){setProvError(e instanceof ApiError?e.message:"Failed to assign provisioning template.")}finally{setProvSaving(false)}}
 async function loadPreview(){if(!device)return;setProvLoading(true);setProvError("");setPreview(null);try{setPreview(await apiFetch<Preview>(`/devices/${device.id}/provisioning/preview`))}catch(e){setProvError(e instanceof ApiError?e.message:"Failed to render provisioning preview.")}finally{setProvLoading(false)}}
 const [allTags,setAllTags]=useState<Tag[]>([]),[deviceTagIds,setDeviceTagIds]=useState<number[]>([]),[tagSaving,setTagSaving]=useState(false),[tagMessage,setTagMessage]=useState("");
 useEffect(()=>{apiFetch<Tag[]>(`/tags?tenantId=${ORG}`).then(setAllTags).catch(()=>{})},[]);
 useEffect(()=>{apiFetch<Tag[]>(`/tag-assignments/device/${id}`).then(list=>setDeviceTagIds(list.map(t=>t.id))).catch(()=>{})},[id]);
 async function toggleTag(tagId:number){const next=deviceTagIds.includes(tagId)?deviceTagIds.filter(t=>t!==tagId):[...deviceTagIds,tagId];setDeviceTagIds(next);setTagSaving(true);setTagMessage("");try{await apiFetch(`/tag-assignments/device/${id}`,{method:"PUT",body:JSON.stringify({tagIds:next})})}catch(e){setTagMessage(e instanceof ApiError?e.message:"Failed to save tags")}finally{setTagSaving(false)}}
 const [trace,setTrace]=useState<TraceResult|null>(null),[tracing,setTracing]=useState(false),[traceError,setTraceError]=useState("");
 async function runTraceroute(){setTracing(true);setTraceError("");setTrace(null);try{setTrace(await apiFetch<TraceResult>(`/devices/${id}/traceroute`,{method:"POST"}))}catch(e){setTraceError(e instanceof ApiError?e.message:"Traceroute failed.")}finally{setTracing(false)}}
 const [icmpSaving,setIcmpSaving]=useState(false),[icmpMessage,setIcmpMessage]=useState("");
 async function saveICMPCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setIcmpSaving(true);setIcmpMessage("");const data=new FormData(e.currentTarget);try{const updated=await apiFetch<Device>(`/devices/${device.id}/icmp-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",intervalSeconds:Number(data.get("intervalSeconds")||30),packetSize:Number(data.get("packetSize")||56),count:Number(data.get("count")||3),retries:Number(data.get("retries")||1)})});setDevice(updated);setIcmpMessage("ICMP ping configuration saved.")}catch(e){setIcmpMessage(e instanceof ApiError?e.message:"Failed to save ICMP ping configuration.")}finally{setIcmpSaving(false)}}
 const [httpSaving,setHttpSaving]=useState(false),[httpMessage,setHttpMessage]=useState("");
 async function saveHTTPCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setHttpSaving(true);setHttpMessage("");const data=new FormData(e.currentTarget);try{const updated=await apiFetch<Device>(`/devices/${device.id}/http-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",url:data.get("url"),expectedStatus:Number(data.get("expectedStatus")||200),keyword:data.get("keyword"),timeoutMs:Number(data.get("timeoutMs")||5000)})});setDevice(updated);setHttpMessage("HTTP check configuration saved.")}catch(e){setHttpMessage(e instanceof ApiError?e.message:"Failed to save HTTP check configuration.")}finally{setHttpSaving(false)}}
 const [dnsSaving,setDnsSaving]=useState(false),[dnsMessage,setDnsMessage]=useState(""),[dnsLive,setDnsLive]=useState<DNSLive|null>(null),[dnsChecking,setDnsChecking]=useState(false);
 async function loadDNSLive(){try{setDnsLive(await apiFetch<DNSLive>(`/dns/${id}/live`))}catch{ /* best-effort */ }}
 useEffect(()=>{loadDNSLive();const t=setInterval(loadDNSLive,15000);return()=>clearInterval(t)},[id]);
 async function checkDNSNow(){setDnsChecking(true);try{await apiFetch(`/dns/${id}/check`,{method:"POST"});await loadDNSLive()}catch{ /* best-effort */ }finally{setDnsChecking(false)}}
 async function saveDNSCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setDnsSaving(true);setDnsMessage("");const data=new FormData(e.currentTarget);try{const updated=await apiFetch<Device>(`/devices/${device.id}/dns-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",hostname:data.get("hostname"),recordType:data.get("recordType"),resolverServer:data.get("resolverServer"),expectedAnswer:data.get("expectedAnswer"),intervalSeconds:Number(data.get("intervalSeconds")||60)})});setDevice(updated);setDnsMessage("DNS check configuration saved.")}catch(e){setDnsMessage(e instanceof ApiError?e.message:"Failed to save DNS check configuration.")}finally{setDnsSaving(false)}}
 const [sshSaving,setSshSaving]=useState(false),[sshMessage,setSshMessage]=useState(""),[sshLive,setSshLive]=useState<ReachLive|null>(null),[sshChecking,setSshChecking]=useState(false);
 async function loadSSHLive(){try{setSshLive(await apiFetch<ReachLive>(`/ssh/${id}/live`))}catch{ /* best-effort */ }}
 useEffect(()=>{loadSSHLive();const t=setInterval(loadSSHLive,15000);return()=>clearInterval(t)},[id]);
 async function checkSSHNow(){setSshChecking(true);try{await apiFetch(`/ssh/${id}/check`,{method:"POST"});await loadSSHLive()}catch{ /* best-effort */ }finally{setSshChecking(false)}}
 async function saveSSHCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setSshSaving(true);setSshMessage("");const data=new FormData(e.currentTarget);try{const updated=await apiFetch<Device>(`/devices/${device.id}/ssh-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",port:Number(data.get("port")||22),bannerKeyword:data.get("bannerKeyword"),timeoutMs:Number(data.get("timeoutMs")||5000),intervalSeconds:Number(data.get("intervalSeconds")||60)})});setDevice(updated);setSshMessage("SSH check configuration saved.")}catch(e){setSshMessage(e instanceof ApiError?e.message:"Failed to save SSH check configuration.")}finally{setSshSaving(false)}}
 const [telnetSaving,setTelnetSaving]=useState(false),[telnetMessage,setTelnetMessage]=useState(""),[telnetLive,setTelnetLive]=useState<ReachLive|null>(null),[telnetChecking,setTelnetChecking]=useState(false);
 async function loadTelnetLive(){try{setTelnetLive(await apiFetch<ReachLive>(`/telnet/${id}/live`))}catch{ /* best-effort */ }}
 useEffect(()=>{loadTelnetLive();const t=setInterval(loadTelnetLive,15000);return()=>clearInterval(t)},[id]);
 async function checkTelnetNow(){setTelnetChecking(true);try{await apiFetch(`/telnet/${id}/check`,{method:"POST"});await loadTelnetLive()}catch{ /* best-effort */ }finally{setTelnetChecking(false)}}
 async function saveTelnetCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setTelnetSaving(true);setTelnetMessage("");const data=new FormData(e.currentTarget);try{const updated=await apiFetch<Device>(`/devices/${device.id}/telnet-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",port:Number(data.get("port")||23),bannerKeyword:data.get("bannerKeyword"),timeoutMs:Number(data.get("timeoutMs")||5000),intervalSeconds:Number(data.get("intervalSeconds")||60)})});setDevice(updated);setTelnetMessage("Telnet check configuration saved.")}catch(e){setTelnetMessage(e instanceof ApiError?e.message:"Failed to save Telnet check configuration.")}finally{setTelnetSaving(false)}}
 const [pushSaving,setPushSaving]=useState(false),[pushMessage,setPushMessage]=useState(""),[copied,setCopied]=useState(false);
 async function savePushCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setPushSaving(true);setPushMessage("");const data=new FormData(e.currentTarget);try{const updated=await apiFetch<Device>(`/devices/${device.id}/push-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",intervalSeconds:Number(data.get("intervalSeconds")||60),gracePeriodSeconds:Number(data.get("gracePeriodSeconds")||30)})});setDevice(updated);setPushMessage("Push monitor configuration saved.")}catch(e){setPushMessage(e instanceof ApiError?e.message:"Failed to save push monitor configuration.")}finally{setPushSaving(false)}}
 function pushURL(token?:string){if(typeof window==="undefined"||!token)return"";return `${window.location.origin}/api/v1/push/${token}?status=up&msg=OK`}
 async function copyPushURL(){if(!device?.pushToken)return;try{await navigator.clipboard.writeText(pushURL(device.pushToken));setCopied(true);setTimeout(()=>setCopied(false),2000)}catch{ /* clipboard unavailable */ }}
 async function discover(){setDiscovering(true);setMessage("");try{const r=await apiFetch<{interfaceCount:number;systemName?:string}>(`/devices/${id}/discover`,{method:"POST"});setMessage(`Discovery completed: ${r.interfaceCount} interfaces saved${r.systemName?` · ${r.systemName}`:""}.`);await load()}catch(e){setMessage(e instanceof ApiError?e.message:e instanceof Error?e.message:"SNMP discovery failed")}finally{setDiscovering(false)}}
 if(loading)return <main className="mx-auto max-w-7xl px-6 py-8 text-slate-400">Loading device…</main>;
 if(!device)return <main className="mx-auto max-w-7xl px-6 py-8"><div className={card}><h1 className="text-xl font-semibold">Device unavailable</h1><p className="mt-2 text-sm text-slate-400">{message||"The requested device does not exist."}</p><Link href="/devices" className="mt-4 inline-block text-cyan-400">← Back to devices</Link></div></main>;
 const up=interfaces.filter(x=>x.operUp).length;
 return <main className="mx-auto max-w-7xl px-6 py-8"><div className="mb-6 flex flex-wrap items-start justify-between gap-4"><div><Link href="/devices" className="text-xs text-slate-500 hover:text-cyan-400">← Network Devices</Link><div className="mt-3 text-xs font-semibold tracking-[.2em] text-cyan-400">DEVICE MONITORING</div><h1 className="mt-1 text-3xl font-bold">{device.name}</h1><p className="mt-1 text-sm text-slate-400">{device.address} · {device.vendor||device.deviceType}</p></div><button onClick={discover} disabled={discovering||!device.snmpEnabled} className="rounded-xl bg-cyan-600 px-5 py-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-40">{discovering?"Discovering…":"Run SNMP Discovery"}</button></div>{message&&<div className="mb-5 rounded-xl border border-cyan-900 bg-cyan-950/30 px-4 py-3 text-sm text-cyan-200">{message}</div>}
 <section className={`mb-6 ${card}`}>
  <h2 className="mb-3 font-semibold">Monitor types on this device</h2>
  <div className="flex flex-wrap gap-2 text-xs">
   {[
    {label:"SNMP",on:device.snmpEnabled},
    {label:"ICMP Ping",on:device.icmpEnabled},
    {label:"HTTP(S) Check",on:device.httpCheckEnabled},
    {label:"DNS Check",on:device.dnsEnabled},
    {label:"SSH Reachability",on:device.sshEnabled},
    {label:"Telnet Reachability",on:device.telnetEnabled},
    {label:"Push Heartbeat",on:device.pushEnabled},
   ].map(m=>(
    <span key={m.label} className={`rounded-full border px-3 py-1 font-medium ${m.on?"border-emerald-800 bg-emerald-950/40 text-emerald-300":"border-slate-800 bg-slate-950 text-slate-600"}`}>
     {m.on?"● ":"○ "}{m.label}
    </span>
   ))}
  </div>
 </section>
 <section className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><h2 className="font-semibold">Tags</h2>{tagSaving&&<span className="text-xs text-slate-500">Saving…</span>}</div>
  {tagMessage&&<div className="mt-2 text-xs text-red-400">{tagMessage}</div>}
  <div className="mt-3 flex flex-wrap gap-2">
   {allTags.length===0&&<span className="text-xs text-slate-500">No tags defined yet — create some on the <Link href="/tags" className="text-cyan-400 hover:underline">Tags</Link> page.</span>}
   {allTags.map(t=>{const on=deviceTagIds.includes(t.id);return <button key={t.id} type="button" onClick={()=>toggleTag(t.id)} className="rounded-full border px-3 py-1 text-xs font-medium transition" style={on?{borderColor:t.color,backgroundColor:`${t.color}22`,color:t.color}:{borderColor:"#30363D",color:"#8B949E"}}>{t.name}</button>})}
  </div>
 </section>
 <section className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4"><div className={card}><div className="text-xs uppercase text-slate-500">SNMP</div><div className="mt-2 text-xl font-bold">{device.snmpEnabled?`Enabled · v${device.snmpVersion}`:"Disabled"}</div><div className="mt-1 text-xs text-slate-500">Port {device.snmpPort}</div></div><div className={card}><div className="text-xs uppercase text-slate-500">Interfaces</div><div className="mt-2 text-xl font-bold">{interfaces.length}</div><div className="mt-1 text-xs text-emerald-400">{up} operationally up</div></div><div className={card}><div className="text-xs uppercase text-slate-500">Errors</div><div className="mt-2 text-xl font-bold">{interfaces.reduce((n,x)=>n+x.inErrors+x.outErrors,0)}</div><div className="mt-1 text-xs text-slate-500">Across discovered interfaces</div></div><div className={card}><div className="text-xs uppercase text-slate-500">Monitoring</div><div className="mt-2 text-xl font-bold">{device.enabled?"Enabled":"Disabled"}</div><div className="mt-1 text-xs text-slate-500">Live inventory view</div></div></section>
 <section className={`mb-6 ${card}`}><h2 className="mb-4 font-semibold">Metric history</h2><div className="grid gap-6 sm:grid-cols-2"><MetricChart subjectType="device" subjectId={id} metric="latency_ms" label="Latency" unit="ms" /><MetricChart subjectType="device" subjectId={id} metric="up" label="Reachability (1=up, 0=down)" formatValue={v=>v.toFixed(0)} /></div></section>
 <section className={`mb-6 ${card}`}>
  <div className="mb-4"><h2 className="font-semibold">HTTP(S) check</h2><p className="mt-1 text-xs text-slate-500">Optional status-code + keyword check against a URL on this device, independent of SNMP/ICMP monitoring. Ported from the previous monitoring setup.</p></div>
  <form onSubmit={saveHTTPCheck} className="grid gap-4 sm:grid-cols-2">
   <label className="sm:col-span-2 flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.httpCheckEnabled} className="h-4 w-4"/><span><b>Enable HTTP check</b><span className="ml-2 text-xs text-slate-500">Poll this URL on the same interval as other device metrics</span></span></label>
   <label className="sm:col-span-2 text-sm text-slate-300">URL<input name="url" defaultValue={device.httpUrl} placeholder="https://192.168.88.1/" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-sm text-slate-300">Expected status code<input name="expectedStatus" type="number" defaultValue={device.httpExpectedStatus||200} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-sm text-slate-300">Timeout (ms)<input name="timeoutMs" type="number" defaultValue={device.httpTimeoutMs||5000} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="sm:col-span-2 text-sm text-slate-300">Keyword to require in response body (optional)<input name="keyword" defaultValue={device.httpKeyword} placeholder={'e.g. "logged in" or a status string'} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <div className="sm:col-span-2"><button disabled={httpSaving} className="rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">{httpSaving?"Saving…":"Save HTTP check"}</button>{httpMessage&&<span className="ml-3 text-xs text-slate-400">{httpMessage}</span>}</div>
  </form>
  {device.httpCheckEnabled&&<div className="mt-6 grid gap-6 sm:grid-cols-2"><MetricChart subjectType="device" subjectId={id} metric="http_latency_ms" label="HTTP latency" unit="ms" /><MetricChart subjectType="device" subjectId={id} metric="http_up" label="HTTP reachability (1=up, 0=down)" formatValue={v=>v.toFixed(0)} /></div>}
 </section>
 <section className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">ICMP Ping</h2><p className="mt-1 text-xs text-slate-500">Round-trip time + packet loss from the periodic ICMP poller. Down/recovery here drives Discord/webhook/email alerts and the browser sound alert.</p></div><button onClick={pingNow} disabled={pingState.probing} className="rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50">{pingState.probing?"Pinging…":"Ping now"}</button></div>
  <form onSubmit={saveICMPCheck} className="mt-4 grid gap-4 rounded-lg border border-slate-800 bg-slate-950 p-4 sm:grid-cols-4">
   <label className="sm:col-span-4 flex items-center gap-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.icmpEnabled} className="h-4 w-4"/><span><b>Enable ICMP ping</b><span className="ml-2 text-xs text-slate-500">Feeds alerting + the sparkline below</span></span></label>
   <label className="text-xs text-slate-400">Interval (s)<input name="intervalSeconds" type="number" min={5} defaultValue={device.icmpIntervalSeconds||30} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Packet size<input name="packetSize" type="number" min={1} defaultValue={device.icmpPacketSize||56} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Packets per probe<input name="count" type="number" min={1} defaultValue={device.icmpCount||3} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Retries before down<input name="retries" type="number" min={1} defaultValue={device.icmpRetries||1} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/><span className="mt-1 block text-[10px] text-slate-600">Consecutive failed cycles before alerting fires (1 = immediately)</span></label>
   <div className="sm:col-span-4"><button disabled={icmpSaving} className="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-semibold hover:bg-cyan-500 disabled:opacity-50">{icmpSaving?"Saving…":"Save ICMP settings"}</button>{icmpMessage&&<span className="ml-3 text-xs text-slate-400">{icmpMessage}</span>}</div>
  </form>
  {pingState.pingError&&<div className="mt-3 text-xs text-red-400">{pingState.pingError}</div>}
  {!pingState.live?<div className="mt-4 text-sm text-slate-500">No ping data yet — waiting for the poller or a manual probe.</div>
   :<div className="mt-4 grid gap-4 sm:grid-cols-4">
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Status</div><div className={`mt-2 text-xl font-bold ${pingState.live.live?.reachable?"text-emerald-400":"text-red-400"}`}>{pingState.live.live?.reachable?"REACHABLE":"UNREACHABLE"}</div></div>
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">RTT avg</div><div className="mt-2 text-xl font-bold">{pingState.live.live&&pingState.live.live.rttMs!=null?`${pingState.live.live.rttMs.toFixed(2)} ms`:"—"}</div></div>
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Loss</div><div className="mt-2 text-xl font-bold">{pingState.live.live&&pingState.live.live.lossPct!=null?`${pingState.live.live.lossPct.toFixed(1)}%`:"—"}</div></div>
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">TTL</div><div className="mt-2 text-xl font-bold">{pingState.live.live?.ttl??"—"}</div></div>
   </div>}
  {pingState.live&&pingState.live.live?.error&&<div className="mt-3 text-xs text-amber-400">{pingState.live.live.error}</div>}
  <div className="mt-4"><PingSparkline results={pingState.live?.history??[]} /></div>
 </section>
 <section className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">DNS Check</h2><p className="mt-1 text-xs text-slate-500">Resolve a hostname against a record type (and, optionally, a specific resolver server), alerting on failure or an unexpected answer. Ported from Uptime Kuma's DNS monitor.</p></div><button onClick={checkDNSNow} disabled={dnsChecking} className="rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50">{dnsChecking?"Checking…":"Check now"}</button></div>
  <form onSubmit={saveDNSCheck} className="mt-4 grid gap-4 rounded-lg border border-slate-800 bg-slate-950 p-4 sm:grid-cols-2">
   <label className="sm:col-span-2 flex items-center gap-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.dnsEnabled} className="h-4 w-4"/><span><b>Enable DNS check</b><span className="ml-2 text-xs text-slate-500">Feeds alerting on resolution failure/mismatch</span></span></label>
   <label className="text-xs text-slate-400">Hostname<input name="hostname" defaultValue={device.dnsHostname} placeholder="example.com" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Record type
    <select name="recordType" defaultValue={device.dnsRecordType||"A"} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500">
     {["A","AAAA","CNAME","MX","TXT","NS","SOA"].map(rt=><option key={rt} value={rt}>{rt}</option>)}
    </select>
   </label>
   <label className="text-xs text-slate-400">Resolver server (optional)<input name="resolverServer" defaultValue={device.dnsResolverServer} placeholder="8.8.8.8 (blank = system default)" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Interval (s)<input name="intervalSeconds" type="number" min={5} defaultValue={device.dnsIntervalSeconds||60} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="sm:col-span-2 text-xs text-slate-400">Expected answer (optional)<input name="expectedAnswer" defaultValue={device.dnsExpectedAnswer} placeholder="e.g. an expected IP/CNAME/text fragment" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <div className="sm:col-span-2"><button disabled={dnsSaving} className="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-semibold hover:bg-cyan-500 disabled:opacity-50">{dnsSaving?"Saving…":"Save DNS check"}</button>{dnsMessage&&<span className="ml-3 text-xs text-slate-400">{dnsMessage}</span>}</div>
  </form>
  {device.dnsEnabled&&<div className="mt-4">
   {!dnsLive?.live?<div className="text-sm text-slate-500">No DNS check data yet — waiting for the poller or a manual check.</div>
    :<div className="grid gap-4 sm:grid-cols-3">
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Status</div><div className={`mt-2 text-xl font-bold ${dnsLive.live.resolved?"text-emerald-400":"text-red-400"}`}>{dnsLive.live.resolved?"RESOLVED":"FAILING"}</div></div>
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Latency</div><div className="mt-2 text-xl font-bold">{dnsLive.live.latencyMs!=null?`${dnsLive.live.latencyMs.toFixed(0)} ms`:"—"}</div></div>
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Answers</div><div className="mt-2 text-sm font-medium text-slate-300">{dnsLive.live.answers?.join(", ")||"—"}</div></div>
    </div>}
   {dnsLive?.live?.error&&<div className="mt-3 text-xs text-amber-400">{dnsLive.live.error}</div>}
  </div>}
 </section>
 <section className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">SSH Reachability</h2><p className="mt-1 text-xs text-slate-500">TCP-connect to the configured port, with an optional identification-banner keyword match (no login is attempted).</p></div><button onClick={checkSSHNow} disabled={sshChecking} className="rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50">{sshChecking?"Checking…":"Check now"}</button></div>
  <form onSubmit={saveSSHCheck} className="mt-4 grid gap-4 rounded-lg border border-slate-800 bg-slate-950 p-4 sm:grid-cols-2">
   <label className="sm:col-span-2 flex items-center gap-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.sshEnabled} className="h-4 w-4"/><span><b>Enable SSH check</b><span className="ml-2 text-xs text-slate-500">Feeds alerting on unreachability</span></span></label>
   <label className="text-xs text-slate-400">Port<input name="port" type="number" defaultValue={device.sshPort||22} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Timeout (ms)<input name="timeoutMs" type="number" defaultValue={device.sshTimeoutMs||5000} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Interval (s)<input name="intervalSeconds" type="number" min={5} defaultValue={device.sshIntervalSeconds||60} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Banner keyword (optional)<input name="bannerKeyword" defaultValue={device.sshBannerKeyword} placeholder="e.g. OpenSSH" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <div className="sm:col-span-2"><button disabled={sshSaving} className="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-semibold hover:bg-cyan-500 disabled:opacity-50">{sshSaving?"Saving…":"Save SSH check"}</button>{sshMessage&&<span className="ml-3 text-xs text-slate-400">{sshMessage}</span>}</div>
  </form>
  {device.sshEnabled&&<div className="mt-4">
   {!sshLive?.live?<div className="text-sm text-slate-500">No SSH check data yet — waiting for the poller or a manual check.</div>
    :<div className="grid gap-4 sm:grid-cols-3">
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Status</div><div className={`mt-2 text-xl font-bold ${sshLive.live.reachable?"text-emerald-400":"text-red-400"}`}>{sshLive.live.reachable?"REACHABLE":"DOWN"}</div></div>
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Latency</div><div className="mt-2 text-xl font-bold">{sshLive.live.latencyMs!=null?`${sshLive.live.latencyMs.toFixed(0)} ms`:"—"}</div></div>
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Banner</div><div className="mt-2 text-sm font-medium text-slate-300">{sshLive.live.banner||"—"}</div></div>
    </div>}
   {sshLive?.live?.error&&<div className="mt-3 text-xs text-amber-400">{sshLive.live.error}</div>}
  </div>}
 </section>
 <section className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">Telnet Reachability</h2><p className="mt-1 text-xs text-slate-500">TCP-connect to the configured port, with an optional banner/login-prompt keyword match (no login is attempted).</p></div><button onClick={checkTelnetNow} disabled={telnetChecking} className="rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50">{telnetChecking?"Checking…":"Check now"}</button></div>
  <form onSubmit={saveTelnetCheck} className="mt-4 grid gap-4 rounded-lg border border-slate-800 bg-slate-950 p-4 sm:grid-cols-2">
   <label className="sm:col-span-2 flex items-center gap-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.telnetEnabled} className="h-4 w-4"/><span><b>Enable Telnet check</b><span className="ml-2 text-xs text-slate-500">Feeds alerting on unreachability</span></span></label>
   <label className="text-xs text-slate-400">Port<input name="port" type="number" defaultValue={device.telnetPort||23} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Timeout (ms)<input name="timeoutMs" type="number" defaultValue={device.telnetTimeoutMs||5000} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Interval (s)<input name="intervalSeconds" type="number" min={5} defaultValue={device.telnetIntervalSeconds||60} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Banner keyword (optional)<input name="bannerKeyword" defaultValue={device.telnetBannerKeyword} placeholder="e.g. login:" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <div className="sm:col-span-2"><button disabled={telnetSaving} className="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-semibold hover:bg-cyan-500 disabled:opacity-50">{telnetSaving?"Saving…":"Save Telnet check"}</button>{telnetMessage&&<span className="ml-3 text-xs text-slate-400">{telnetMessage}</span>}</div>
  </form>
  {device.telnetEnabled&&<div className="mt-4">
   {!telnetLive?.live?<div className="text-sm text-slate-500">No Telnet check data yet — waiting for the poller or a manual check.</div>
    :<div className="grid gap-4 sm:grid-cols-3">
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Status</div><div className={`mt-2 text-xl font-bold ${telnetLive.live.reachable?"text-emerald-400":"text-red-400"}`}>{telnetLive.live.reachable?"REACHABLE":"DOWN"}</div></div>
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Latency</div><div className="mt-2 text-xl font-bold">{telnetLive.live.latencyMs!=null?`${telnetLive.live.latencyMs.toFixed(0)} ms`:"—"}</div></div>
     <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Banner</div><div className="mt-2 text-sm font-medium text-slate-300">{telnetLive.live.banner||"—"}</div></div>
    </div>}
   {telnetLive?.live?.error&&<div className="mt-3 text-xs text-amber-400">{telnetLive.live.error}</div>}
  </div>}
 </section>
 <section className={`mb-6 ${card}`}>
  <div className="mb-4"><h2 className="font-semibold">Push Monitor (heartbeat)</h2><p className="mt-1 text-xs text-slate-500">The monitored thing calls RoutingNMS on its own schedule instead of being polled — point a cron job at the URL below. Down if no push arrives within interval + grace period. Ported from Uptime Kuma's Push monitor.</p></div>
  <form onSubmit={savePushCheck} className="grid gap-4 rounded-lg border border-slate-800 bg-slate-950 p-4 sm:grid-cols-2">
   <label className="sm:col-span-2 flex items-center gap-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.pushEnabled} className="h-4 w-4"/><span><b>Enable push monitor</b><span className="ml-2 text-xs text-slate-500">Generates a push URL the first time you enable it</span></span></label>
   <label className="text-xs text-slate-400">Expected interval (s)<input name="intervalSeconds" type="number" min={10} defaultValue={device.pushIntervalSeconds||60} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-xs text-slate-400">Grace period (s)<input name="gracePeriodSeconds" type="number" min={0} defaultValue={device.pushGracePeriodSeconds??30} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm outline-none focus:border-cyan-500"/></label>
   <div className="sm:col-span-2"><button disabled={pushSaving} className="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-semibold hover:bg-cyan-500 disabled:opacity-50">{pushSaving?"Saving…":"Save push monitor"}</button>{pushMessage&&<span className="ml-3 text-xs text-slate-400">{pushMessage}</span>}</div>
  </form>
  {device.pushToken&&<div className="mt-4">
   <div className="text-xs uppercase text-slate-500">Push URL</div>
   <div className="mt-1 flex flex-wrap items-center gap-2">
    <code className="flex-1 overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 px-3 py-2 text-xs text-emerald-300">{pushURL(device.pushToken)}</code>
    <button type="button" onClick={copyPushURL} className="rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40">{copied?"Copied!":"Copy"}</button>
   </div>
   <div className="mt-3 grid gap-4 sm:grid-cols-3">
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Last push</div><div className="mt-2 text-sm font-bold">{device.pushLastSeenAt?new Date(device.pushLastSeenAt).toLocaleString():"Never"}</div></div>
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Last status</div><div className="mt-2 text-sm font-bold">{device.pushLastStatus||"—"}</div></div>
    <div className="rounded-xl border border-slate-800 bg-slate-950 p-4"><div className="text-xs uppercase text-slate-500">Last message</div><div className="mt-2 text-sm font-medium text-slate-300">{device.pushLastMessage||"—"}</div></div>
   </div>
  </div>}
 </section>
 <section id="traceroute" className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">Traceroute</h2><p className="mt-1 text-xs text-slate-500">On-demand hop-by-hop path trace to this device — an advanced diagnostic the previous monitoring setup never offered.</p></div><button onClick={runTraceroute} disabled={tracing} className="rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50">{tracing?"Tracing…":"Run traceroute"}</button></div>
  {traceError&&<div className="mt-3 text-xs text-red-400">{traceError}</div>}
  {trace&&<div className="mt-4 overflow-x-auto">
   {trace.error&&<div className="mb-3 text-xs text-amber-400">{trace.error}</div>}
   <table className="w-full text-left text-sm">
    <thead><tr className="text-xs uppercase text-slate-500"><th className="pb-2 pr-4">Hop</th><th className="pb-2 pr-4">Address</th><th className="pb-2">RTT</th></tr></thead>
    <tbody>
     {trace.hops.map(h=>(
      <tr key={h.number} className="border-t border-slate-800">
       <td className="py-1.5 pr-4 text-slate-500">{h.number}</td>
       <td className="py-1.5 pr-4">{h.timedOut?<span className="text-slate-600">* * *</span>:<span>{h.hostname?`${h.hostname} `:""}<span className="text-slate-500">{h.address}</span></span>}</td>
       <td className="py-1.5">{h.rttMs!=null?`${h.rttMs.toFixed(2)} ms`:"—"}</td>
      </tr>
     ))}
    </tbody>
   </table>
  </div>}
  {!trace&&!tracing&&<div className="mt-4 text-sm text-slate-500">Run a trace to see the path to this device, hop by hop.</div>}
 </section>
 {device.deviceType==="router"&&<section className={`mb-6 ${card}`}>
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">RouterOS auto-provisioning</h2><p className="mt-1 text-xs text-slate-500">Assign a script template; the router pulls its own config via <code>/tool fetch</code> using a serial-derived password.</p></div></div>
  {!device.serialNumber&&<div className="mt-4 rounded-lg border border-amber-900 bg-amber-950/30 px-4 py-3 text-sm text-amber-300">Add a serial number to this device (edit its registration) before it can be provisioned.</div>}
  <div className="mt-4 grid gap-4 sm:grid-cols-2">
   <label className="text-sm text-slate-300">Provisioning template
    <select value={device.provisioningTemplateId??""} disabled={provSaving||!device.serialNumber} onChange={e=>assignTemplate(e.target.value)} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500">
     <option value="">None assigned</option>
     {templates.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}
    </select>
   </label>
   <div className="text-sm text-slate-300"><div className="text-xs uppercase text-slate-500">Last provisioned</div><div className="mt-1">{device.lastProvisionedAt?new Date(device.lastProvisionedAt).toLocaleString():"Never"}</div></div>
  </div>
  {device.provisioningTemplateId&&device.serialNumber&&<button onClick={loadPreview} disabled={provLoading} className="mt-4 rounded-lg border border-cyan-800 bg-cyan-950/40 px-4 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50">{provLoading?"Rendering…":"Preview rendered script + fetch command"}</button>}
  {provError&&<div className="mt-3 text-xs text-red-400">{provError}</div>}
  {preview&&<div className="mt-4 space-y-3">
   <div><div className="text-xs uppercase text-slate-500">RouterOS fetch command</div><pre className="mt-1 overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-3 text-xs text-emerald-300">{preview.fetchCommand}</pre></div>
   <div><div className="text-xs uppercase text-slate-500">Derived admin password</div><pre className="mt-1 overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-3 text-xs text-cyan-300">{preview.password}</pre></div>
   <div><div className="text-xs uppercase text-slate-500">Rendered script</div><pre className="mt-1 overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-3 text-xs text-slate-300">{preview.renderedScript}</pre></div>
  </div>}
 </section>}
 <section className={card}><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold">Interface inventory</h2><p className="mt-1 text-xs text-slate-500">IF-MIB data discovered from the device and persisted in PostgreSQL.</p></div><button onClick={load} className="rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button></div><div className="mt-5 overflow-x-auto"><table className="w-full min-w-[900px] text-left text-sm"><thead className="border-b border-slate-800 text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-3 py-3">Index</th><th>Name</th><th>Description</th><th>Admin</th><th>Oper</th><th>In errors</th><th>Out errors</th><th>Last discovery</th></tr></thead><tbody>{interfaces.length?interfaces.map(x=><tr key={x.id} className="border-b border-slate-800/70"><td className="px-3 py-3 text-slate-500">{x.ifIndex}</td><td className="font-medium">{x.name||"—"}</td><td className="text-slate-400">{x.description||"—"}</td><td><span className={x.adminUp?"text-emerald-400":"text-slate-500"}>{x.adminUp?"UP":"DOWN"}</span></td><td><span className={x.operUp?"text-emerald-400":"text-red-400"}>{x.operUp?"UP":"DOWN"}</span></td><td>{x.inErrors}</td><td>{x.outErrors}</td><td className="text-xs text-slate-500">{x.lastDiscoveredAt?new Date(x.lastDiscoveredAt).toLocaleString():"—"}</td></tr>):<tr><td colSpan={8} className="py-12 text-center text-slate-500">No interface inventory yet. Click <b>Run SNMP Discovery</b> to discover and save interfaces.</td></tr>}</tbody></table></div> </section></main>
}

/** Lightweight RTT sparkline over recent ICMP probe history (hand-rolled SVG,
 *  matching the MetricChart approach). Missing RTT (a failed probe) plots a
 *  red dot at the bottom so reachability dips are visible between samples. */
function PingSparkline({ results }: { results: PingResult[] }) {
  if (results.length === 0) {
    return <div className="text-sm text-slate-500">No ping history yet.</div>;
  }
  const width = 480, height = 72, pad = 4;
  const rtts = results.map(r => r.rttMs ?? 0);
  const max = Math.max(...rtts, 1);
  const stepX = results.length > 1 ? (width - pad * 2) / (results.length - 1) : 0;
  const points = results.map((r, i) => {
    const x = pad + i * stepX;
    const y = r.isReachable && r.rttMs != null ? height - pad - (r.rttMs / max) * (height - pad * 2) : height - pad;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  return (
    <div>
      <div className="mb-1 text-xs uppercase text-slate-500">Ping RTT history (last {results.length} probes)</div>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full" preserveAspectRatio="none">
        <polyline points={points.join(" ")} fill="none" stroke="#22d3ee" strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
        {results.map((r, i) => r.isReachable ? null : (
          <circle key={i} cx={(pad + i * stepX).toFixed(1)} cy={height - pad} r={2.5} fill="#f87171" vectorEffect="non-scaling-stroke" />
        ))}
      </svg>
      <div className="flex justify-between text-[10px] text-slate-600">
        <span>0 ms</span>
        <span>{max.toFixed(0)} ms</span>
      </div>
    </div>
  );
}
