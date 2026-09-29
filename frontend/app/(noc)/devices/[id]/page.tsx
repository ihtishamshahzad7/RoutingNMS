"use client";

import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LineChart, Line, ResponsiveContainer, XAxis, YAxis, Tooltip, CartesianGrid } from "recharts";
import { RotateCw } from "lucide-react";
import { ApiError, apiFetch } from "../../../../lib/api";
import { MetricChart } from "../../../../components/metric-chart";
import { EngPanel, EngButton, EngModal, ENG } from "../../../../components/ui/engineer";
import { HeartbeatBar, type Beat } from "../../dashboard/HeartbeatBar";
import { aggregate, type RawPoint } from "../../../../lib/monitoring-aggregate";

// Item 3.4 (per-port history graphs): the Ports section below and its
// history modal reuse the exact same building blocks as the Connectivity
// Monitoring detail page's charts (frontend/app/(noc)/icmp-monitoring/[id]/
// page.tsx) -- HeartbeatBar for the up/down strip and lib/monitoring-
// aggregate's aggregate() for the 1h/24h/7d bucketing -- rather than
// rebuilding either, per the standing "reuse, don't rebuild" instruction.

type CertChainLink={certType:string;subject:string;issuer:string;validFrom:string;validTo:string;fingerprintSha256:string};
type CertInfo={subject:string;issuer:string;validFrom:string;validTo:string;fingerprintSha256:string;daysRemaining:number;chain:CertChainLink[]};
type Device={id:string;name:string;address:string;deviceType:string;vendor?:string;serialNumber?:string;enabled:boolean;snmpEnabled:boolean;snmpVersion:string;snmpPort:number;snmpConfigured:boolean;provisioningTemplateId?:number|null;lastProvisionedAt?:string;httpCheckEnabled:boolean;httpUrl?:string;httpExpectedStatus:number;httpKeyword?:string;httpTimeoutMs:number;httpMethod:string;httpBody?:string;httpBodyEncoding:string;httpHeaders?:string;httpAcceptedStatusCodes:string;httpMaxRedirects:number;httpIgnoreTls:boolean;certInfo?:CertInfo|null;icmpEnabled:boolean;icmpIntervalSeconds:number;icmpPacketSize:number;icmpCount:number;icmpRetries:number;dnsEnabled:boolean;dnsHostname?:string;dnsRecordType:string;dnsResolverServer?:string;dnsExpectedAnswer?:string;dnsIntervalSeconds:number;pushEnabled:boolean;pushToken?:string;pushIntervalSeconds:number;pushGracePeriodSeconds:number;pushLastSeenAt?:string;pushLastStatus?:string;pushLastMessage?:string;sshEnabled:boolean;sshPort:number;sshBannerKeyword?:string;sshTimeoutMs:number;sshIntervalSeconds:number;telnetEnabled:boolean;telnetPort:number;telnetBannerKeyword?:string;telnetTimeoutMs:number;telnetIntervalSeconds:number};
type DNSLive={live:{resolved:boolean;answers?:string[];latencyMs:number;expectedMatch?:boolean|null;error?:string}};
type ReachLive={live:{reachable:boolean;banner?:string;latencyMs:number;bannerMatched?:boolean|null;error?:string}};
type ProvTemplate={id:number;name:string;scriptBody:string};
type Preview={renderedScript:string;password:string;fetchCommand:string};
type Interface={id:number;deviceId:string;ifIndex:number;name:string;description:string;adminUp:boolean;operUp:boolean;inOctets:number;outOctets:number;inErrors:number;outErrors:number;lastDiscoveredAt?:string;speedBps?:number;counterWidth?:number;inRateBps?:number|null;outRateBps?:number|null;lastTransitionAt?:string;snmpReachable?:boolean;snmpLastSuccessAt?:string;snmpLastError?:string};

// Item 3.4: per-port history point (GET /api/v1/interfaces/{id}/history-range).
type PortHistoryPoint={probedAt:string;inRateBps?:number|null;outRateBps?:number|null;operUp:boolean};
type PortRange="1h"|"24h"|"7d";
const PORT_RANGES:{key:PortRange;label:string}[]=[{key:"1h",label:"1h"},{key:"24h",label:"24h"},{key:"7d",label:"7d"}];

type PortStatus="up"|"down"|"admin-down";
function portStatus(x:Interface):PortStatus{ if(!x.adminUp) return "admin-down"; return x.operUp?"up":"down"; }
const PORT_STATUS_COLOR:Record<PortStatus,string>={up:ENG.up,down:ENG.down,"admin-down":"#8A96A3"};
const PORT_STATUS_LABEL:Record<PortStatus,string>={up:"Up",down:"Down","admin-down":"Admin down"};
// Down ports first (the thing an engineer needs to see), then admin-down
// (intentionally disabled, not an active problem), then up -- matches the
// Events page's severity-first ordering convention.
const PORT_RANK:Record<PortStatus,number>={down:0,"admin-down":1,up:2};

function formatBps(v?:number|null):string{
  if(v==null) return "—";
  if(v>=1_000_000_000) return `${(v/1_000_000_000).toFixed(2)} Gbps`;
  if(v>=1_000_000) return `${(v/1_000_000).toFixed(2)} Mbps`;
  if(v>=1_000) return `${(v/1_000).toFixed(1)} Kbps`;
  return `${v.toFixed(0)} bps`;
}

/** Per-port history view (item 3.4): in/out traffic charts + an up/down
 * strip, reusing the same 1h/24h/7d range selector, aggregate() bucketing,
 * and HeartbeatBar already built for ICMP and Port/Service checks. */
function PortHistoryModal({ port, onClose }: { port: Interface; onClose: () => void }) {
  const [range, setRange] = useState<PortRange>("24h");
  const [history, setHistory] = useState<PortHistoryPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadHistory = useCallback(() => {
    let active = true;
    setLoading(true);
    apiFetch<{ history: PortHistoryPoint[] }>(`/interfaces/${port.id}/history-range?range=${range}`)
      .then((r) => { if (active) { setHistory(r.history); setError(""); } })
      .catch((e) => { if (active) setError(e instanceof ApiError ? e.message : "Unable to load port history."); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [port.id, range]);

  useEffect(() => loadHistory(), [loadHistory]);

  const inAgg = useMemo(
    () => aggregate(history.map((p): RawPoint => ({ t: new Date(p.probedAt).getTime(), latencyMs: p.inRateBps ?? null, reachable: p.operUp })), range),
    [history, range]
  );
  const outAgg = useMemo(
    () => aggregate(history.map((p): RawPoint => ({ t: new Date(p.probedAt).getTime(), latencyMs: p.outRateBps ?? null, reachable: p.operUp })), range),
    [history, range]
  );
  const beats: Beat[] = useMemo(() => history.map((p) => ({ reachable: p.operUp, probedAt: p.probedAt })), [history]);

  const axisCommon = {
    stroke: "#5C6B7A",
    tick: { fontSize: 10, fill: "#5C6B7A" },
    axisLine: { stroke: "#DCE1E8" },
    tickLine: { stroke: "#DCE1E8" },
  };

  return (
    <EngModal
      title={`${port.name || `Interface ${port.ifIndex}`} · history`}
      subtitle={port.description || undefined}
      onClose={onClose}
      footer={<EngButton onClick={onClose}>Close</EngButton>}
    >
      <div className="mb-3 flex justify-end">
        <div className="flex rounded-[4px] border border-[#DCE1E8] bg-white p-0.5">
          {PORT_RANGES.map((r) => (
            <button
              key={r.key}
              onClick={() => setRange(r.key)}
              className={`rounded-[3px] px-3 py-1 text-[12px] font-medium transition-colors duration-150 ${
                range === r.key ? "bg-[#2E7BF6] text-white" : "text-[#5C6B7A] hover:bg-[#F4F6F9]"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>
      {loading ? (
        <div className="py-10 text-center text-[13px] text-[#8A96A3]">Loading…</div>
      ) : error ? (
        <div className="flex flex-col items-center gap-2 py-10 text-center">
          <span className="text-[13px] text-[#C4362D]">{error}</span>
          <button
            onClick={loadHistory}
            className="inline-flex items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-3 py-1.5 text-[12px] font-medium text-[#1F2A37] hover:bg-[#F4F6F9]"
          >
            <RotateCw size={12} /> Retry
          </button>
        </div>
      ) : history.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-[#8A96A3]">No history in this range yet.</div>
      ) : (
        <>
          <div className="mb-4">
            <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-[#5C6B7A]">In traffic</div>
            <ResponsiveContainer width="100%" height={140}>
              <LineChart data={inAgg.map((p) => ({ t: p.t, v: p.avgLatencyMs }))}>
                <CartesianGrid stroke="#EEF1F4" vertical={false} />
                <XAxis dataKey="t" type="number" domain={["dataMin", "dataMax"]} tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} {...axisCommon} />
                <YAxis width={56} tickFormatter={(v) => formatBps(v)} {...axisCommon} />
                <Tooltip
                  contentStyle={{ background: "#FFFFFF", border: "1px solid #DCE1E8", borderRadius: 4, fontSize: 12, fontFamily: "inherit" }}
                  labelFormatter={(t) => new Date(t as number).toLocaleString()}
                  formatter={(v) => [formatBps(v as number), "In"]}
                />
                <Line type="monotone" dataKey="v" stroke="#2E7BF6" strokeWidth={1.5} dot={false} isAnimationActive={false} connectNulls={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
          <div className="mb-4">
            <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-[#5C6B7A]">Out traffic</div>
            <ResponsiveContainer width="100%" height={140}>
              <LineChart data={outAgg.map((p) => ({ t: p.t, v: p.avgLatencyMs }))}>
                <CartesianGrid stroke="#EEF1F4" vertical={false} />
                <XAxis dataKey="t" type="number" domain={["dataMin", "dataMax"]} tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} {...axisCommon} />
                <YAxis width={56} tickFormatter={(v) => formatBps(v)} {...axisCommon} />
                <Tooltip
                  contentStyle={{ background: "#FFFFFF", border: "1px solid #DCE1E8", borderRadius: 4, fontSize: 12, fontFamily: "inherit" }}
                  labelFormatter={(t) => new Date(t as number).toLocaleString()}
                  formatter={(v) => [formatBps(v as number), "Out"]}
                />
                <Line type="monotone" dataKey="v" stroke="#1E8E5A" strokeWidth={1.5} dot={false} isAnimationActive={false} connectNulls={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
          <div>
            <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-[#5C6B7A]">Up / down</div>
            <div className="overflow-x-auto py-1">
              <HeartbeatBar beats={beats} height={24} />
            </div>
          </div>
        </>
      )}
    </EngModal>
  );
}
type PingResult={id:number;deviceId:string;probedAt:string;rttMs?:number|null;jitterMs?:number|null;lossPct:number;ttl?:number|null;isReachable:boolean};
type PingLive={live?:{address?:string;reachable:boolean;rttMs?:number;jitterMs?:number;lossPct?:number;ttl?:number;error?:string};history:PingResult[]};
type Tag={id:number;name:string;color:string};
type TraceHop={number:number;address?:string;hostname?:string;rttMs?:number|null;timedOut:boolean};
type TraceResult={address:string;hops:TraceHop[];ranAt:string;error?:string};

const ORG="tenant-1";
const card="rounded-2xl border border-slate-800 bg-slate-900 p-5";

export default function DeviceDetailsPage(){
  const searchParams = useSearchParams();
  const highlightIfIndex = searchParams.get("highlight");
  const highlightedRowRef = useRef<HTMLTableRowElement | null>(null);
 const params=useParams<{id:string}>(); const id=params.id;
 const [device,setDevice]=useState<Device|null>(null),[interfaces,setInterfaces]=useState<Interface[]>([]),[loading,setLoading]=useState(true),[discovering,setDiscovering]=useState(false),[message,setMessage]=useState("");
  const [portsLoading, setPortsLoading] = useState(true);
  const [portsError, setPortsError] = useState("");
  const [historyPort, setHistoryPort] = useState<Interface | null>(null);
  async function loadInterfaces() {
    setPortsLoading(true);
    setPortsError("");
    try {
      setInterfaces(await apiFetch<Interface[]>(`/devices/${id}/interfaces`));
    } catch (e) {
      setPortsError(e instanceof ApiError ? e.message : "Unable to load port data.");
    } finally {
      setPortsLoading(false);
    }
  }
  useEffect(() => {
    if (highlightIfIndex && highlightedRowRef.current) {
      highlightedRowRef.current.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  }, [highlightIfIndex, interfaces]);
 const [templates,setTemplates]=useState<ProvTemplate[]>([]),[preview,setPreview]=useState<Preview|null>(null),[provSaving,setProvSaving]=useState(false),[provLoading,setProvLoading]=useState(false),[provError,setProvError]=useState("");
 const [pingState,setPingState]=useState<{live:PingLive|null;probing:boolean;pingError:string}>({live:null,probing:false,pingError:""});
 async function loadPing(){try{const live=await apiFetch<PingLive>(`/ping/${id}/live`);setPingState(s=>({...s,live,pingError:""}))}catch(e){setPingState(s=>({...s,live:null,pingError:e instanceof ApiError?e.message:"Unable to load ping status."}))}}
 async function pingNow(){setPingState(s=>({...s,probing:true,pingError:""}));try{await apiFetch(`/ping/${id}/probe`,{method:"POST"});await loadPing()}catch(e){setPingState(s=>({...s,probing:false,pingError:e instanceof ApiError?e.message:"Ping probe failed."}))}finally{setPingState(s=>({...s,probing:false}))}}
 async function load(){setLoading(true);try{const devices=await apiFetch<Device[]>(`/devices?organizationId=${ORG}`);const d=devices.find(x=>x.id===id);if(!d)throw new Error("Device not found");setDevice(d);await loadInterfaces()}catch(e){setMessage(e instanceof ApiError?e.message:e instanceof Error?e.message:"Unable to load device")}finally{setLoading(false)}}
 async function togglePause(){if(!device)return;try{const updated=await apiFetch<Device>(`/devices/${device.id}/pause`,{method:"PUT",body:JSON.stringify({enabled:!device.enabled})});setDevice(updated);setMessage(updated.enabled?"Resumed — monitoring is active again.":"Paused — monitoring and alerting stopped, configuration kept.")}catch(e){setMessage(e instanceof ApiError?e.message:"Failed to update device state.")}}
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
 async function saveHTTPCheck(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!device)return;setHttpSaving(true);setHttpMessage("");const data=new FormData(e.currentTarget);try{const acceptedStatusCodes=String(data.get("acceptedStatusCodes")||"").split(",").map(s=>s.trim()).filter(Boolean);const updated=await apiFetch<Device>(`/devices/${device.id}/http-check`,{method:"PUT",body:JSON.stringify({enabled:data.get("enabled")==="on",url:data.get("url"),expectedStatus:Number(data.get("expectedStatus")||200),keyword:data.get("keyword"),timeoutMs:Number(data.get("timeoutMs")||5000),method:data.get("method")||"GET",bodyEncoding:data.get("bodyEncoding")||"json",body:data.get("body"),headers:data.get("headers"),acceptedStatusCodes,maxRedirects:Number(data.get("maxRedirects")||10),ignoreTls:data.get("ignoreTls")==="on"})});setDevice(updated);setHttpMessage("HTTP check configuration saved.")}catch(e){setHttpMessage(e instanceof ApiError?e.message:"Failed to save HTTP check configuration.")}finally{setHttpSaving(false)}}
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
 return <main className="mx-auto max-w-7xl px-6 py-8"><div className="mb-6 flex flex-wrap items-start justify-between gap-4"><div><Link href="/devices" className="text-xs text-slate-500 hover:text-cyan-400">← Network Devices</Link><div className="mt-3 text-xs font-semibold tracking-[.2em] text-cyan-400">DEVICE MONITORING</div><h1 className="mt-1 text-3xl font-bold">{device.name}</h1><p className="mt-1 text-sm text-slate-400">{device.address} · {device.vendor||device.deviceType}</p></div><div className="flex gap-3"><button onClick={togglePause} title={device.enabled?"Pause monitoring and alerting for this device without deleting it":"Resume monitoring for this device"} className={`rounded-xl border px-5 py-3 text-sm font-semibold ${device.enabled?"border-amber-800 bg-amber-950/30 text-amber-300 hover:bg-amber-950/50":"border-emerald-800 bg-emerald-950/30 text-emerald-300 hover:bg-emerald-950/50"}`}>{device.enabled?"Pause monitoring":"Resume monitoring"}</button><button onClick={discover} disabled={discovering||!device.snmpEnabled} className="rounded-xl bg-cyan-600 px-5 py-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-40">{discovering?"Discovering…":"Run SNMP Discovery"}</button></div></div>{message&&<div className="mb-5 rounded-xl border border-cyan-900 bg-cyan-950/30 px-4 py-3 text-sm text-cyan-200">{message}</div>}
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
 <section className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4"><div className={card}><div className="text-xs uppercase text-slate-500">SNMP</div><div className="mt-2 text-xl font-bold">{device.snmpEnabled?`Enabled · v${device.snmpVersion}`:"Disabled"}</div><div className="mt-1 text-xs text-slate-500">Port {device.snmpPort}</div></div><div className={card}><div className="text-xs uppercase text-slate-500">Interfaces</div><div className="mt-2 text-xl font-bold">{interfaces.length}</div><div className="mt-1 text-xs text-emerald-400">{up} operationally up</div></div><div className={card}><div className="text-xs uppercase text-slate-500">Errors</div><div className="mt-2 text-xl font-bold">{interfaces.reduce((n,x)=>n+x.inErrors+x.outErrors,0)}</div><div className="mt-1 text-xs text-slate-500">Across discovered interfaces</div></div><div className={card}><div className="text-xs uppercase text-slate-500">Monitoring</div><div className={`mt-2 text-xl font-bold ${device.enabled?"":"text-amber-400"}`}>{device.enabled?"Active":"Paused"}</div><div className="mt-1 text-xs text-slate-500">{device.enabled?"Live inventory view":"Checks and alerting are stopped"}</div></div></section>
 <section className={`mb-6 ${card}`}><h2 className="mb-4 font-semibold">Metric history</h2><div className="grid gap-6 sm:grid-cols-2"><MetricChart subjectType="device" subjectId={id} metric="latency_ms" label="Latency" unit="ms" /><MetricChart subjectType="device" subjectId={id} metric="up" label="Reachability (1=up, 0=down)" formatValue={v=>v.toFixed(0)} /></div></section>
 <section className={`mb-6 ${card}`}>
  <div className="mb-4"><h2 className="font-semibold">HTTP(S) check</h2><p className="mt-1 text-xs text-slate-500">Uptime Kuma-parity HTTP(s) monitor: method, headers, body, accepted status codes, follow-redirect, and ignore-TLS-error, matching Uptime Kuma&apos;s real EditMonitor fields (Kuma core-parity checklist, Group A/B item 1).</p></div>
  <form onSubmit={saveHTTPCheck} className="grid gap-4 sm:grid-cols-2">
   <label className="sm:col-span-2 flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm"><input name="enabled" type="checkbox" defaultChecked={device.httpCheckEnabled} className="h-4 w-4"/><span><b>Enable HTTP check</b><span className="ml-2 text-xs text-slate-500">Poll this URL on the same interval as other device metrics</span></span></label>
   <label className="sm:col-span-2 text-sm text-slate-300">URL<input name="url" defaultValue={device.httpUrl} placeholder="https://192.168.88.1/" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-sm text-slate-300">Method<select name="method" defaultValue={device.httpMethod||"GET"} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500">{["GET","POST","PUT","PATCH","DELETE","HEAD","OPTIONS"].map(m=><option key={m} value={m}>{m}</option>)}</select></label>
   <label className="text-sm text-slate-300">Body encoding<select name="bodyEncoding" defaultValue={device.httpBodyEncoding||"json"} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"><option value="json">JSON</option><option value="xml">XML</option></select></label>
   <label className="text-sm text-slate-300">Timeout (ms)<input name="timeoutMs" type="number" defaultValue={device.httpTimeoutMs||5000} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="text-sm text-slate-300">Max. redirects (0 = don&apos;t follow)<input name="maxRedirects" type="number" min={0} defaultValue={device.httpMaxRedirects??10} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="sm:col-span-2 text-sm text-slate-300">Accepted status codes (comma-separated ranges or exact codes)<input name="acceptedStatusCodes" defaultValue={device.httpAcceptedStatusCodes||"200-299"} placeholder="200-299, 301, 404" className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/><span className="mt-1 block text-xs text-slate-500">Falls back to &quot;Expected status code&quot; below if left empty.</span></label>
   <label className="text-sm text-slate-300">Expected status code (fallback, used only when accepted status codes above is empty)<input name="expectedStatus" type="number" defaultValue={device.httpExpectedStatus||200} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <label className="flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm"><input name="ignoreTls" type="checkbox" defaultChecked={device.httpIgnoreTls} className="h-4 w-4"/><span><b>Ignore TLS error</b><span className="ml-2 text-xs text-slate-500">Skip certificate verification (self-signed devices)</span></span></label>
   <label className="sm:col-span-2 text-sm text-slate-300">Headers (JSON object, optional)<textarea name="headers" defaultValue={device.httpHeaders} placeholder={'{\n  "HeaderName": "HeaderValue"\n}'} rows={3} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm font-mono outline-none focus:border-cyan-500"/></label>
   <label className="sm:col-span-2 text-sm text-slate-300">Body (optional)<textarea name="body" defaultValue={device.httpBody} placeholder={'{\n  "key": "value"\n}'} rows={3} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm font-mono outline-none focus:border-cyan-500"/></label>
   <label className="sm:col-span-2 text-sm text-slate-300">Keyword to require in response body (optional)<input name="keyword" defaultValue={device.httpKeyword} placeholder={'e.g. "logged in" or a status string'} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none focus:border-cyan-500"/></label>
   <div className="sm:col-span-2"><button disabled={httpSaving} className="rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">{httpSaving?"Saving…":"Save HTTP check"}</button>{httpMessage&&<span className="ml-3 text-xs text-slate-400">{httpMessage}</span>}</div>
  </form>
  {device.httpCheckEnabled&&<div className="mt-6 grid gap-6 sm:grid-cols-2"><MetricChart subjectType="device" subjectId={id} metric="http_latency_ms" label="HTTP latency" unit="ms" /><MetricChart subjectType="device" subjectId={id} metric="http_up" label="HTTP reachability (1=up, 0=down)" formatValue={v=>v.toFixed(0)} /></div>}
 </section>
 {device.httpCheckEnabled&&device.certInfo&&(()=>{const ci=device.certInfo!;const daysColor=ci.daysRemaining<7?"text-red-400":ci.daysRemaining<30?"text-amber-400":"text-emerald-400";const fmt=(s:string)=>{try{return new Date(s).toLocaleDateString(undefined,{year:"numeric",month:"short",day:"numeric"})}catch{return s}};return(
 <section className={`mb-6 ${card}`}>
  <div className="mb-4"><h2 className="font-semibold">Certificate Info</h2><p className="mt-1 text-xs text-slate-500">TLS certificate details captured from the most recent HTTP(S) check, ported from Uptime Kuma&apos;s monitor certificate panel.</p></div>
  <div className="grid gap-4 sm:grid-cols-2">
   <div><div className="text-xs uppercase text-slate-500">Subject</div><div className="mt-1 text-sm">{ci.subject}</div></div>
   <div><div className="text-xs uppercase text-slate-500">Issuer</div><div className="mt-1 text-sm">{ci.issuer}</div></div>
   <div><div className="text-xs uppercase text-slate-500">Valid from</div><div className="mt-1 text-sm">{fmt(ci.validFrom)}</div></div>
   <div><div className="text-xs uppercase text-slate-500">Valid to</div><div className="mt-1 text-sm">{fmt(ci.validTo)}</div></div>
   <div><div className="text-xs uppercase text-slate-500">Days remaining</div><div className={`mt-1 text-sm font-semibold ${daysColor}`}>{ci.daysRemaining}d</div></div>
   <div><div className="text-xs uppercase text-slate-500">Fingerprint (SHA-256)</div><div className="mt-1 truncate font-mono text-xs" title={ci.fingerprintSha256}>{ci.fingerprintSha256.slice(0,16)}…</div></div>
  </div>
  {ci.chain?.length>0&&<div className="mt-4"><div className="mb-2 text-xs uppercase text-slate-500">Chain</div><ul className="space-y-1 text-sm">{ci.chain.map((c,i)=>(<li key={i} className="rounded-lg border border-slate-800 bg-slate-950 px-3 py-2"><span className="text-slate-500">{c.certType}</span> — {c.subject}</li>))}</ul></div>}
 </section>
 )})()}
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
 <ConfigBackupsSection device={device} />
 <BadgesSection device={device} />
 {device.snmpEnabled && (() => {
   const sortedPorts = [...interfaces].sort((a, b) => {
     const ra = PORT_RANK[portStatus(a)], rb = PORT_RANK[portStatus(b)];
     if (ra !== rb) return ra - rb;
     return (a.name || `if${a.ifIndex}`).localeCompare(b.name || `if${b.ifIndex}`);
   });
   return (
     <EngPanel
       title={`Ports (${interfaces.length})`}
       actions={<EngButton onClick={loadInterfaces}><RotateCw size={13} /> Refresh</EngButton>}
       className="mt-6"
     >
       <p className="mb-3 -mt-1 text-[11px] text-[#8A96A3]">IF-MIB data discovered from the device and persisted in PostgreSQL. Click a port for its traffic and up/down history.</p>
       {portsLoading ? (
         <div className="animate-pulse space-y-2">
           {Array.from({ length: 5 }).map((_, i) => (
             <div key={i} className="h-9 rounded-[3px] bg-[#EEF1F4]" />
           ))}
         </div>
       ) : portsError ? (
         <div className="flex flex-col items-center gap-2 py-10 text-center">
           <span className="text-[13px] text-[#C4362D]">{portsError}</span>
           <button
             onClick={loadInterfaces}
             className="inline-flex items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-3 py-1.5 text-[12px] font-medium text-[#1F2A37] hover:bg-[#F4F6F9]"
           >
             <RotateCw size={12} /> Retry
           </button>
         </div>
       ) : sortedPorts.length === 0 ? (
         <div className="py-10 text-center text-[13px] text-[#8A96A3]">No SNMP interface data yet.</div>
       ) : (
         <div className="overflow-x-auto">
           <table className="w-full min-w-[820px] text-left text-[13px]">
             <thead className="border-b border-[#DCE1E8] text-[11px] uppercase tracking-wide text-[#8A96A3]">
               <tr>
                 <th className="py-2 pl-2 font-medium">Port</th>
                 <th className="font-medium">Admin</th>
                 <th className="font-medium">Oper</th>
                 <th className="font-medium">Speed</th>
                 <th className="font-medium">In</th>
                 <th className="font-medium">Out</th>
                 <th className="font-medium">Last transition</th>
               </tr>
             </thead>
             <tbody>
               {sortedPorts.map((x) => {
                 const status = portStatus(x);
                 const matchesHighlight = !!highlightIfIndex && (x.name === highlightIfIndex || String(x.ifIndex) === highlightIfIndex || `if${x.ifIndex}` === highlightIfIndex);
                 return (
                   <tr
                     key={x.id}
                     id={`interface-${x.ifIndex}`}
                     ref={matchesHighlight ? highlightedRowRef : undefined}
                     onClick={() => setHistoryPort(x)}
                     className={`cursor-pointer border-b border-[#EEF1F4] transition-colors duration-1000 hover:bg-[#F4F6F9] ${matchesHighlight ? "bg-amber-100" : ""}`}
                   >
                     <td className="py-2 pl-2">
                       <span className="mr-2 inline-block h-2 w-2 rounded-full align-middle" style={{ background: PORT_STATUS_COLOR[status] }} />
                       <span className="font-medium text-[#1F2A37]">{x.name || `if${x.ifIndex}`}</span>
                       {x.description && <span className="ml-1 text-[#8A96A3]">· {x.description}</span>}
                     </td>
                     <td><span style={{ color: x.adminUp ? ENG.up : "#8A96A3" }}>{x.adminUp ? "Up" : "Down"}</span></td>
                     <td><span style={{ color: PORT_STATUS_COLOR[status] }}>{PORT_STATUS_LABEL[status]}</span></td>
                     <td className="font-mono text-[#5C6B7A]">{x.speedBps ? formatBps(x.speedBps) : "—"}</td>
                     <td className="font-mono text-[#2E7BF6]">{formatBps(x.inRateBps)}</td>
                     <td className="font-mono text-[#1E8E5A]">{formatBps(x.outRateBps)}</td>
                     <td className="text-[#8A96A3]">{x.lastTransitionAt ? new Date(x.lastTransitionAt).toLocaleString() : "—"}</td>
                   </tr>
                 );
               })}
             </tbody>
           </table>
         </div>
       )}
     </EngPanel>
   );
 })()}
 {historyPort && <PortHistoryModal port={historyPort} onClose={() => setHistoryPort(null)} />}
 </main>
}

/** Feature 1.6 (Config Backup): version history of this device's own
 *  running-config exports, with manual paste/upload, a line diff between
 *  any two versions, and (for routers) the RouterOS scheduler script that
 *  auto-pushes a fresh export on a recurring schedule. Restoring a stored
 *  version back onto the device is deliberately not offered here -- see
 *  the package doc comment in backend/internal/configbackup for why. */
type ConfigBackup = { id: number; deviceId: string; byteSize: number; sha256: string; source: string; takenAt: string; configText?: string };
function ConfigBackupsSection({ device }: { device: Device }) {
  const [items, setItems] = useState<ConfigBackup[]>([]);
  const [loadErr, setLoadErr] = useState("");
  const [pasteText, setPasteText] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState("");
  const [fromId, setFromId] = useState<number | "">("");
  const [toId, setToId] = useState<number | "">("");
  const [diffLines, setDiffLines] = useState<{ op: string; text: string }[] | null>(null);
  const [diffErr, setDiffErr] = useState("");
  const [setup, setSetup] = useState<{ schedulerScript: string } | null>(null);
  const [setupErr, setSetupErr] = useState("");

  async function load() {
    try { setItems(await apiFetch<ConfigBackup[]>(`/devices/${device.id}/config-backups`)); setLoadErr(""); }
    catch (e) { setLoadErr(e instanceof ApiError ? e.message : "Unable to load config backups."); }
  }
  useEffect(() => { load(); }, [device.id]);

  async function savePaste() {
    if (!pasteText.trim()) return;
    setSaving(true); setSaveMsg("");
    try {
      await apiFetch(`/devices/${device.id}/config-backups`, { method: "POST", body: JSON.stringify({ configText: pasteText }) });
      setPasteText(""); setSaveMsg("Saved a new config backup version."); await load();
    } catch (e) { setSaveMsg(e instanceof ApiError ? e.message : "Failed to save config backup."); }
    finally { setSaving(false); }
  }

  async function runDiff() {
    if (fromId === "" || toId === "") return;
    setDiffErr(""); setDiffLines(null);
    try {
      const r = await apiFetch<{ lines: { op: string; text: string }[] }>(`/devices/${device.id}/config-backups/diff?from=${fromId}&to=${toId}`);
      setDiffLines(r.lines);
    } catch (e) { setDiffErr(e instanceof ApiError ? e.message : "Failed to compute diff."); }
  }

  async function loadSetup() {
    setSetupErr(""); setSetup(null);
    try { setSetup(await apiFetch<{ schedulerScript: string }>(`/devices/${device.id}/config-backups/setup`)); }
    catch (e) { setSetupErr(e instanceof ApiError ? e.message : "Failed to render scheduler script (does this device have a serial number?)."); }
  }

  return (
    <section className={`mb-6 ${card}`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div><h2 className="font-semibold">Config Backup</h2><p className="mt-1 text-xs text-slate-500">Version history of this device&apos;s own running config. Paste/upload a version manually, or (RouterOS) schedule it to push automatically. Restoring a version back onto the device isn&apos;t supported here yet.</p></div>
        {device.deviceType === "router" && <button onClick={loadSetup} className="rounded-lg border border-cyan-800 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40">Show auto-backup scheduler script</button>}
      </div>
      {setupErr && <div className="mt-3 text-xs text-red-400">{setupErr}</div>}
      {setup && <pre className="mt-3 overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-3 text-xs text-emerald-300">{setup.schedulerScript}</pre>}

      <div className="mt-4 grid gap-3">
        <textarea value={pasteText} onChange={e => setPasteText(e.target.value)} rows={4} placeholder="Paste a config export (e.g. RouterOS /export output) to save it as a new version…" className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs outline-none focus:border-cyan-500" />
        <div><button onClick={savePaste} disabled={saving || !pasteText.trim()} className="rounded-lg bg-cyan-600 px-4 py-2 text-xs font-semibold hover:bg-cyan-500 disabled:opacity-50">{saving ? "Saving…" : "Save as new version"}</button>{saveMsg && <span className="ml-3 text-xs text-slate-400">{saveMsg}</span>}</div>
      </div>

      {loadErr && <div className="mt-3 text-xs text-red-400">{loadErr}</div>}
      {items.length === 0 ? (
        <div className="mt-4 text-sm text-slate-500">No config backups yet.</div>
      ) : (
        <div className="mt-4 overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead><tr className="text-xs uppercase text-slate-500"><th className="pb-2 pr-4">Taken</th><th className="pb-2 pr-4">Source</th><th className="pb-2 pr-4">Size</th><th className="pb-2">Diff</th></tr></thead>
            <tbody>
              {items.map(b => (
                <tr key={b.id} className="border-t border-slate-800">
                  <td className="py-1.5 pr-4">{new Date(b.takenAt).toLocaleString()}</td>
                  <td className="py-1.5 pr-4 text-slate-400">{b.source}</td>
                  <td className="py-1.5 pr-4 text-slate-500">{b.byteSize} B</td>
                  <td className="py-1.5"><label className="mr-3 text-xs"><input type="radio" name="cb-from" className="mr-1" checked={fromId === b.id} onChange={() => setFromId(b.id)} />from</label><label className="text-xs"><input type="radio" name="cb-to" className="mr-1" checked={toId === b.id} onChange={() => setToId(b.id)} />to</label></td>
                </tr>
              ))}
            </tbody>
          </table>
          <button onClick={runDiff} disabled={fromId === "" || toId === ""} className="mt-3 rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800 disabled:opacity-50">Compare selected versions</button>
        </div>
      )}
      {diffErr && <div className="mt-3 text-xs text-red-400">{diffErr}</div>}
      {diffLines && (
        <pre className="mt-3 max-h-96 overflow-auto rounded-lg border border-slate-800 bg-slate-950 p-3 text-xs">
          {diffLines.map((l, i) => (
            <div key={i} className={l.op === "add" ? "text-emerald-400" : l.op === "remove" ? "text-red-400" : "text-slate-500"}>
              {l.op === "add" ? "+ " : l.op === "remove" ? "- " : "  "}{l.text}
            </div>
          ))}
        </pre>
      )}
    </section>
  );
}

/** Embeddable SVG status-badge URLs for this device (ported from Uptime
 *  Kuma's dynamic badge feature) -- shows the copyable badge URL plus a live
 *  `<img>` preview for each badge type. These endpoints are public
 *  (unauthenticated) and only ever return real data once this device is
 *  added as an item on a *published* status page; until then every badge
 *  still loads fine, it just shows a grey "N/A". The cert-expiry badge is
 *  only offered for devices with HTTP(S) checking enabled, since that's the
 *  only source of certificate-expiry data. */
function BadgesSection({ device }: { device: Device }) {
  const [origin, setOrigin] = useState("");
  useEffect(() => { if (typeof window !== "undefined") setOrigin(window.location.origin); }, []);
  const base = `${origin}/api/v1/badge/${device.id}`;
  const badges: { key: string; label: string; path: string }[] = [
    { key: "status", label: "Status", path: `${base}/status` },
    { key: "uptime", label: "Uptime (24h)", path: `${base}/uptime/24h` },
    { key: "ping", label: "Ping", path: `${base}/ping` },
    { key: "avg-response", label: "Avg Response (24h)", path: `${base}/avg-response/24h` },
    { key: "response", label: "Response", path: `${base}/response` },
  ];
  if (device.httpCheckEnabled) {
    badges.push({ key: "cert-exp", label: "Cert Expiry", path: `${base}/cert-exp` });
  }
  return (
    <section className={`mb-6 ${card}`}>
      <div className="mb-1"><h2 className="font-semibold">Badges</h2></div>
      <p className="mt-1 text-xs text-slate-500">Embeddable status badges for READMEs, wikis or dashboards — public URLs, no login required. Only show real data once this device is added to a <b>published</b> status page; otherwise they render a neutral &quot;N/A&quot;.</p>
      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        {badges.map(b => (
          <div key={b.key} className="rounded-xl border border-slate-800 bg-slate-950 p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="text-xs uppercase text-slate-500">{b.label}</div>
              {origin && <img src={b.path} alt={`${b.label} badge`} className="h-5" />}
            </div>
            <div className="mt-3 flex items-center gap-2">
              <code className="flex-1 overflow-x-auto rounded-lg border border-slate-800 bg-slate-900 px-3 py-2 text-xs text-emerald-300">{b.path}</code>
              <CopyButton text={b.path} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      onClick={async () => { try { await navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000); } catch { /* clipboard unavailable */ } }}
      className="shrink-0 rounded-lg border border-cyan-700 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40"
    >{copied ? "Copied!" : "Copy"}</button>
  );
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
