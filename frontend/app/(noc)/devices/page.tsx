"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "../../../lib/api";
import { groupSections, GroupSectionRows } from "../../../lib/device-groups";
import { Activity, CheckCircle2, Copy, Loader2, Network, Pause, Play, Plus, RefreshCw, Router, Search, Server, ShieldCheck, Wifi, X, Zap } from "lucide-react";
import { EngModal, EngSection, EngField, EngInput, EngSelect, EngToggle, EngButton, isValidIPv4 } from "../../../components/ui/engineer";

type Device={id:string;name:string;address:string;deviceType:string;vendor?:string;serialNumber?:string;enabled:boolean;snmpEnabled:boolean;snmpVersion:string;snmpPort:number;snmpConfigured:boolean};
type TestResult={reachable:boolean;systemName?:string;sysDescr?:string;interfaceCount?:number;error?:string};
type ScanFound={address:string;systemName?:string;sysDescr?:string;deviceType:string;vendor?:string};
type ScanJob={id:string;cidr:string;status:string;total:number;scanned:number;results:ScanFound[];error?:string};
type DeviceGroup={id:number;name:string;sortOrder:number};
type GroupMember={groupId:number;subjectType:string;subjectId:string;sortOrder:number};
const ORG="tenant-1";
const input="mt-1 h-[33px] w-full rounded-[4px] border border-[#DCE1E8] bg-white px-2.5 text-[13px] text-[#1F2A37] outline-none transition-colors duration-150 focus:border-[#2E7BF6] placeholder:text-[#9AA6B2]";
const card="rounded-[4px] border border-[#DCE1E8] bg-white";

export default function DevicesPage(){
 const [devices,setDevices]=useState<Device[]>([]),[loading,setLoading]=useState(true),[saving,setSaving]=useState(false),[testing,setTesting]=useState(false),[message,setMessage]=useState(""),[result,setResult]=useState<TestResult|null>(null),[selected,setSelected]=useState<Device|null>(null);
 const [scanJob,setScanJob]=useState<ScanJob|null>(null),[scanning,setScanning]=useState(false),[scanError,setScanError]=useState(""),[selectedAddrs,setSelectedAddrs]=useState<Set<string>>(new Set()),[importing,setImporting]=useState(false);
 const [groups,setGroups]=useState<DeviceGroup[]>([]),[memberOf,setMemberOf]=useState<Record<string,number>>({}),[collapsed,setCollapsed]=useState<Set<string>>(new Set());
 const [checked,setChecked]=useState<Set<string>>(new Set()),[bulkBusy,setBulkBusy]=useState(false),[showAdd,setShowAdd]=useState(false),[showTest,setShowTest]=useState(false),[showDiscovery,setShowDiscovery]=useState(false);
 const [creatingGroup,setCreatingGroup]=useState(false);
 const [addressError,setAddressError]=useState("");
 const [portCheckOn,setPortCheckOn]=useState(false),[portCheckProtocol,setPortCheckProtocol]=useState("tcp");
 const addFormRef=useRef<HTMLFormElement>(null);
 async function load(){setLoading(true);try{setDevices(await apiFetch<Device[]>(`/devices?organizationId=${ORG}`))}catch(e){setMessage(e instanceof ApiError?e.message:"Unable to load devices.")}finally{setLoading(false)}}
 useEffect(()=>{load()},[]);
 async function loadGroups(){try{const[gs,ms]=await Promise.all([apiFetch<DeviceGroup[]>(`/device-groups?tenantId=${ORG}`),apiFetch<GroupMember[]>("/device-groups/members")]);setGroups(gs);const map:Record<string,number>={};ms.filter(m=>m.subjectType==="device").forEach(m=>{map[m.subjectId]=m.groupId});setMemberOf(map)}catch{/* optional overlay */}}
 useEffect(()=>{loadGroups()},[]);
 async function assignGroup(d:Device,groupId:number|null){try{await apiFetch(`/device-group-assignments/device/${d.id}`,{method:"PUT",body:JSON.stringify({groupId,sortOrder:0})});await loadGroups()}catch(err){setMessage(err instanceof ApiError?err.message:"Failed to update device group.")}}
 function toggleCollapsed(key:string){setCollapsed(prev=>{const n=new Set(prev);n.has(key)?n.delete(key):n.add(key);return n})}
 function toggleChecked(id:string){setChecked(prev=>{const n=new Set(prev);n.has(id)?n.delete(id):n.add(id);return n})}
 async function bulkPause(enabled:boolean){if(!checked.size)return;setBulkBusy(true);try{await apiFetch("/devices/pause-bulk",{method:"PUT",body:JSON.stringify({ids:Array.from(checked),enabled})});setMessage(`${checked.size} device(s) ${enabled?"resumed":"paused"}.`);setChecked(new Set());await load()}catch(err){setMessage(err instanceof ApiError?err.message:"Failed to update selected devices.")}finally{setBulkBusy(false)}}
 async function togglePause(d:Device){try{await apiFetch(`/devices/${d.id}/pause`,{method:"PUT",body:JSON.stringify({enabled:!d.enabled})});setMessage(`${d.name} ${d.enabled?"paused":"resumed"}.`);await load()}catch(err){setMessage(err instanceof ApiError?err.message:"Failed to update device state.")}}
 function cloneDevice(d:Device){setShowAdd(true);setTimeout(()=>{const f=addFormRef.current;if(!f)return;(f.elements.namedItem("name") as HTMLInputElement).value=`${d.name} (Clone)`;(f.elements.namedItem("address") as HTMLInputElement).value=d.address;(f.elements.namedItem("deviceType") as HTMLSelectElement).value=d.deviceType;(f.elements.namedItem("vendor") as HTMLInputElement).value=d.vendor||"";},0)}
 function makePayload(data:FormData){const version=String(data.get("version")||"2c");return {organizationId:ORG,name:data.get("name"),address:data.get("address"),deviceType:data.get("deviceType"),vendor:data.get("vendor"),serialNumber:data.get("serialNumber"),snmpPort:Number(data.get("snmpPort")||161),timeoutMs:Number(data.get("timeoutMs")||3000),snmp:{version,community:data.get("community"),username:data.get("username"),authProto:data.get("authProto"),authPass:data.get("authPass"),privProto:data.get("privProto"),privPass:data.get("privPass")}}}
 async function add(e:FormEvent<HTMLFormElement>){
  e.preventDefault();
  const addrVal=String(new FormData(e.currentTarget).get("address")||"");
  if(!isValidIPv4(addrVal)){setAddressError("Enter a valid IPv4 address (e.g. 192.168.88.17).");return}
  setSaving(true);
  const form=e.currentTarget;
  const data=new FormData(form);
  const groupSel=String(data.get("groupSelect")||"");
  const newGroupName=String(data.get("newGroupName")||"").trim();
  const icmpEnabled=data.get("icmpEnabled")==="on";
  const snmpEnabled=data.get("snmpEnabled")==="on";
  const portCheckEnabled=data.get("portCheckEnabled")==="on";
  const payload={
    organizationId:ORG,
    name:data.get("name"),
    address:data.get("address"),
    deviceType:"router",
    vendor:"",
    serialNumber:"",
    icmpEnabled,
    icmpIntervalSeconds:Number(data.get("icmpIntervalSeconds")||30),
    snmpEnabled,
    snmpPort:Number(data.get("snmpPort")||161),
    snmp:{version:data.get("snmpVersion")||"2c",community:data.get("community")},
    portCheckEnabled,
    portCheckProtocol:data.get("portCheckProtocol")||"tcp",
    portCheckPort:Number(data.get("portCheckPort")||0),
    portCheckPath:data.get("portCheckPath")||"/",
    portCheckAcceptedStatusCodes:data.get("portCheckAcceptedStatusCodes")||"200-299",
    portCheckIntervalSeconds:Number(data.get("portCheckIntervalSeconds")||60),
  };
  try{
    const d=await apiFetch<Device>("/devices",{method:"POST",body:JSON.stringify(payload)});
    // Group assignment: either an existing group picked from the dropdown, or a
    // brand-new one typed into "+ Create new group…" -- created first, then the
    // device is assigned to it, same PUT the Devices table's own per-row group
    // selector already uses (assignGroup above).
    let groupId:number|null=null;
    if(groupSel==="__new__"&&newGroupName){
      const g=await apiFetch<DeviceGroup>("/device-groups",{method:"POST",body:JSON.stringify({tenantId:ORG,name:newGroupName,sortOrder:groups.length})});
      groupId=g.id;
    }else if(groupSel){
      groupId=Number(groupSel);
    }
    if(groupId!=null){
      await apiFetch(`/device-group-assignments/device/${d.id}`,{method:"PUT",body:JSON.stringify({groupId,sortOrder:0})});
    }
    setMessage(`✓ ${d.name} registered${icmpEnabled?" · ICMP monitoring started":""}${snmpEnabled?" · SNMP enabled":""}${portCheckEnabled?" · Port/Service check started":""}.`);
    form.reset();
    setCreatingGroup(false);
    setAddressError("");
    setPortCheckOn(false);
    setPortCheckProtocol("tcp");
    setShowAdd(false);
    await Promise.all([load(),loadGroups()]);
  }catch(err){
    setMessage(err instanceof ApiError?err.message:"Failed to register device.");
  }finally{
    setSaving(false);
  }
 }
 async function test(e:FormEvent<HTMLFormElement>){e.preventDefault();setTesting(true);setResult(null);try{setResult(await apiFetch<TestResult>("/devices/test",{method:"POST",body:JSON.stringify(makePayload(new FormData(e.currentTarget)))}))}catch(err){setResult({reachable:false,error:err instanceof ApiError?err.message:"SNMP test failed."})}finally{setTesting(false)}}
 async function saveSNMP(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!selected)return;setSaving(true);try{const d=new FormData(e.currentTarget);await apiFetch(`/devices/${selected.id}/snmp`,{method:"PUT",body:JSON.stringify({enabled:d.get("enabled")==="on",version:d.get("version"),community:d.get("community"),username:d.get("username"),authProto:d.get("authProto"),authPass:d.get("authPass"),privProto:d.get("privProto"),privPass:d.get("privPass"),port:Number(d.get("port")||161),timeoutMs:Number(d.get("timeoutMs")||3000)})});setMessage(`✓ SNMP configuration saved for ${selected.name}.`);setSelected(null);await load()}catch(err){setMessage(err instanceof ApiError?err.message:"Failed to save SNMP configuration.")}finally{setSaving(false)}}
 async function startScan(e:FormEvent<HTMLFormElement>){e.preventDefault();setScanning(true);setScanError("");setScanJob(null);setSelectedAddrs(new Set());try{const d=new FormData(e.currentTarget);setScanJob(await apiFetch<ScanJob>("/discovery/scan",{method:"POST",body:JSON.stringify({cidr:d.get("cidr"),version:d.get("scanVersion")||"2c",community:d.get("scanCommunity")||"public",port:Number(d.get("scanPort")||161),timeoutMs:Number(d.get("scanTimeoutMs")||1500)})}))}catch(err){setScanError(err instanceof ApiError?err.message:"Failed to start subnet scan.")}finally{setScanning(false)}}
 useEffect(()=>{if(!scanJob||scanJob.status!=="running")return;const id=scanJob.id;const t=window.setInterval(async()=>{try{setScanJob(await apiFetch<ScanJob>(`/discovery/scan/${id}`))}catch{/* transient */}},1500);return()=>window.clearInterval(t)},[scanJob?.id,scanJob?.status]);
 async function importSelected(){if(!scanJob||!selectedAddrs.size)return;setImporting(true);try{const r=await apiFetch<{created:Device[];failed?:string[]}>("/discovery/import",{method:"POST",body:JSON.stringify({jobId:scanJob.id,organizationId:ORG,addresses:Array.from(selectedAddrs)})});setMessage(`Imported ${r.created.length} device(s) from discovery${r.failed?.length?` · ${r.failed.length} failed`:""}.`);setScanJob(p=>p?{...p,results:p.results.filter(x=>!selectedAddrs.has(x.address))}:p);setSelectedAddrs(new Set());await load()}catch(err){setScanError(err instanceof ApiError?err.message:"Failed to import devices.")}finally{setImporting(false)}}
 const stats=useMemo(()=>({total:devices.length,monitoring:devices.filter(d=>d.enabled).length,snmp:devices.filter(d=>d.snmpConfigured).length,attention:devices.filter(d=>!d.snmpConfigured).length}),[devices]);
 const groupsView=groupSections(devices,groups,memberOf);
 return <main className="mx-auto max-w-[1500px] px-4 py-6 sm:px-6 lg:px-8">
  <header className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between"><div><div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[.18em] text-[#2E7BF6]"><Network size={14}/> Infrastructure / Devices</div><h1 className="text-3xl font-bold tracking-tight text-[#1F2A37]">Network device management</h1><p className="mt-2 max-w-3xl text-sm leading-6 text-[#5C6B7A]">One professional workspace to register infrastructure, configure SNMP, verify connectivity and move devices into active monitoring.</p></div><div className="flex gap-2"><button onClick={()=>setShowTest(true)} className="inline-flex items-center gap-2 rounded-lg border border-[#DCE1E8] bg-white px-4 py-2.5 text-sm font-semibold text-[#1F2A37] hover:border-[#DCE1E8] hover:bg-[#EEF1F4]"><ShieldCheck size={16}/> Test SNMP</button><button onClick={()=>setShowDiscovery(v=>!v)} className="inline-flex items-center gap-2 rounded-lg border border-[#DCE1E8] bg-white px-4 py-2.5 text-sm font-semibold text-[#1F2A37] hover:border-[#DCE1E8] hover:bg-[#EEF1F4]"><Search size={16}/> Discover</button><button onClick={()=>{setAddressError("");setPortCheckOn(false);setPortCheckProtocol("tcp");setShowAdd(true)}} className="inline-flex items-center gap-2 rounded-lg bg-[#2E7BF6] px-4 py-2.5 text-sm font-medium text-white hover:bg-[#2568D4]"><Plus size={17}/> Add device</button></div></header>
  {message&&<div className="mb-5 flex items-center justify-between rounded-lg border border-[#BFD7FB] bg-[#EEF3FD] px-4 py-3 text-sm text-[#1D4ED8]"><span>{message}</span><button onClick={()=>setMessage("")}><X size={15}/></button></div>}
  <div className="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><Stat icon={Server} label="Total devices" value={stats.total} hint="Registered infrastructure"/><Stat icon={Activity} label="Monitoring active" value={stats.monitoring} hint={`${stats.total?Math.round(stats.monitoring/stats.total*100):0}% of inventory`} good/><Stat icon={ShieldCheck} label="SNMP configured" value={stats.snmp} hint="Ready for polling" good/><Stat icon={Zap} label="Needs attention" value={stats.attention} hint="SNMP not configured" warn={stats.attention>0}/></div>
  {showDiscovery&&<section className={`${card} mb-6 overflow-hidden`}><div className="flex items-center justify-between border-b border-[#DCE1E8] px-5 py-4"><div><h2 className="font-semibold text-[#1F2A37]">Subnet discovery</h2><p className="mt-1 text-xs text-[#8A96A3]">Find SNMP-responsive routers, switches, OLTs and servers before registering them.</p></div><button onClick={()=>setShowDiscovery(false)} className="text-[#8A96A3] hover:text-[#1F2A37]"><X size={18}/></button></div><form onSubmit={startScan} className="grid gap-4 p-5 md:grid-cols-6"><Field label="CIDR" wide><input required name="cidr" placeholder="192.168.88.0/24" className={input}/></Field><Field label="Version"><select name="scanVersion" className={input}><option value="2c">SNMP v2c</option></select></Field><Field label="Community"><input name="scanCommunity" type="password" placeholder="public" className={input}/></Field><Field label="Port"><input name="scanPort" type="number" defaultValue="161" className={input}/></Field><Field label="Timeout"><input name="scanTimeoutMs" type="number" defaultValue="1500" className={input}/></Field><div className="md:col-span-6"><button disabled={scanning} className="inline-flex items-center gap-2 rounded-lg bg-white px-4 py-2.5 text-sm font-bold text-[#1F2A37] disabled:opacity-50">{scanning?<Loader2 size={16} className="animate-spin"/>:<Search size={16}/>} {scanning?"Starting scan…":"Scan network"}</button></div></form>{scanError&&<div className="mx-5 mb-5 rounded-lg border border-[#F0C2C2] bg-[#FBEAEA] px-4 py-3 text-sm text-[#C4362D]">{scanError}</div>}{scanJob&&<div className="border-t border-[#DCE1E8] p-5"><div className="mb-3 flex flex-wrap items-center justify-between gap-2 text-xs text-[#5C6B7A]"><span>{scanJob.cidr} · {scanJob.status==="running"?`Scanning ${scanJob.scanned}/${scanJob.total}`:`Complete · ${scanJob.results.length} responsive host(s)`}</span>{selectedAddrs.size>0&&<button onClick={importSelected} disabled={importing} className="rounded-lg bg-[#1E8E5A] px-3 py-2 text-xs font-medium text-white">{importing?"Importing…":`Import ${selectedAddrs.size} selected`}</button>}</div>{scanJob.status==="running"&&<div className="mb-4 h-1.5 overflow-hidden rounded-full bg-[#EEF1F4]"><div className="h-full bg-[#2568D4] transition-all" style={{width:`${scanJob.total?Math.round(scanJob.scanned/scanJob.total*100):0}%`}}/></div>}<div className="overflow-x-auto"><table className="w-full text-left text-xs"><thead className="text-[#8A96A3]"><tr><th className="pb-2 w-8"></th><th>Address</th><th>System</th><th>Type</th><th>Vendor</th></tr></thead><tbody>{scanJob.results.map(f=><tr key={f.address} className="border-t border-[#EEF1F4]"><td className="py-2"><input type="checkbox" checked={selectedAddrs.has(f.address)} onChange={()=>setSelectedAddrs(p=>{const n=new Set(p);n.has(f.address)?n.delete(f.address):n.add(f.address);return n})}/></td><td className="py-2 font-mono">{f.address}</td><td className="py-2">{f.systemName||"—"}</td><td className="py-2 uppercase text-[#8A96A3]">{f.deviceType}</td><td className="py-2 text-[#5C6B7A]">{f.vendor||"—"}</td></tr>)}</tbody></table></div></div>}</section>}
  <section className={card}><div className="flex flex-col gap-3 border-b border-[#DCE1E8] px-5 py-4 md:flex-row md:items-center md:justify-between"><div><h2 className="font-semibold text-[#1F2A37]">Device inventory</h2><p className="mt-1 text-xs text-[#8A96A3]">Select a device to monitor it. Use <b>Add SNMP</b> when credentials are not yet configured.</p></div><div className="flex flex-wrap items-center gap-2">{checked.size>0&&<><span className="text-xs text-[#8A96A3]">{checked.size} selected</span><button onClick={()=>bulkPause(false)} disabled={bulkBusy} className="rounded-md border border-[#DCE1E8] px-3 py-2 text-xs text-[#C77700]">Pause</button><button onClick={()=>bulkPause(true)} disabled={bulkBusy} className="rounded-md border border-[#DCE1E8] px-3 py-2 text-xs text-[#1E8E5A]">Resume</button></>}<button onClick={load} className="inline-flex items-center gap-2 rounded-md border border-[#DCE1E8] px-3 py-2 text-xs text-[#5C6B7A] hover:bg-[#EEF1F4]"><RefreshCw size={14}/> Refresh</button></div></div><div className="overflow-x-auto"><table className="w-full min-w-[1050px] text-left text-sm"><thead className="border-b border-[#DCE1E8] bg-[#F9FAFC] text-[11px] uppercase tracking-wider text-[#8A96A3]"><tr><th className="w-10 px-5 py-3"></th><th className="px-3 py-3">Device</th><th className="px-3 py-3">Address</th><th className="px-3 py-3">Role</th><th className="px-3 py-3">Vendor</th><th className="px-3 py-3">SNMP</th><th className="px-3 py-3">Monitor</th><th className="px-3 py-3">Group</th><th className="px-5 py-3 text-right">Actions</th></tr></thead><tbody>{loading?<tr><td colSpan={9} className="py-14 text-center text-[#8A96A3]"><Loader2 className="mx-auto mb-2 animate-spin" size={20}/>Loading inventory…</td></tr>:devices.length?groupsView.map(section=><GroupSectionRows key={section.key} section={section} collapsed={collapsed.has(section.key)} onToggle={()=>toggleCollapsed(section.key)} render={d=><tr key={d.id} className="border-b border-[#EEF1F4] hover:bg-[#F9FAFC]"><td className="px-5 py-3"><input type="checkbox" checked={checked.has(d.id)} onChange={()=>toggleChecked(d.id)} className="h-4 w-4 rounded border-[#DCE1E8] bg-[#F9FAFC]"/></td><td className="px-3 py-3"><Link href={`/devices/${d.id}`} className="flex items-center gap-3"><DeviceIcon type={d.deviceType}/><span><span className="block font-semibold text-[#1F2A37] hover:text-[#2E7BF6]">{d.name}</span><span className="block text-[11px] text-[#8A96A3]">{d.serialNumber||"No serial recorded"}</span></span></Link></td><td className="px-3 py-3 font-mono text-xs text-[#5C6B7A]">{d.address}</td><td className="px-3 py-3 text-xs uppercase text-[#8A96A3]">{d.deviceType}</td><td className="px-3 py-3 text-xs text-[#5C6B7A]">{d.vendor||"—"}</td><td className="px-3 py-3">{d.snmpConfigured?<span className="inline-flex items-center gap-1.5 rounded-full border border-[#BFE3D0] bg-[#E9F5EF] px-2.5 py-1 text-[10px] font-bold text-[#1E8E5A]"><CheckCircle2 size={12}/> v{d.snmpVersion}</span>:<span className="inline-flex rounded-full border border-[#F3D9A8] bg-[#FDF3E4] px-2.5 py-1 text-[10px] font-bold text-[#C77700]">Not configured</span>}</td><td className="px-3 py-3"><span className={`inline-flex rounded-full px-2.5 py-1 text-[10px] font-bold ${d.enabled?"bg-[#E9F5EF] text-[#1E8E5A]":"bg-[#EEF1F4] text-[#8A96A3]"}`}>{d.enabled?"ACTIVE":"PAUSED"}</span></td><td className="px-3 py-3"><select aria-label={`Group for ${d.name}`} value={memberOf[d.id]??""} onChange={e=>assignGroup(d,e.target.value?Number(e.target.value):null)} className="rounded-md border border-[#DCE1E8] bg-[#F9FAFC] px-2 py-1.5 text-[11px] text-[#5C6B7A]"><option value="">Ungrouped</option>{groups.map(g=><option key={g.id} value={g.id}>{g.name}</option>)}</select></td><td className="px-5 py-3"><div className="flex justify-end gap-1.5"><Link href={`/devices/${d.id}`} className="rounded-md bg-[#2E7BF6] px-3 py-1.5 text-[11px] font-medium text-white hover:bg-[#2568D4]">Open</Link><button onClick={()=>setSelected(d)} className="rounded-md border border-[#DCE1E8] px-3 py-1.5 text-[11px] font-semibold text-[#1F2A37] hover:bg-[#EEF1F4]">{d.snmpConfigured?"SNMP":"Add SNMP"}</button><button onClick={()=>cloneDevice(d)} title="Clone device" className="rounded-md border border-[#DCE1E8] p-1.5 text-[#8A96A3] hover:text-[#1F2A37]"><Copy size={14}/></button><button onClick={()=>togglePause(d)} title={d.enabled?"Pause monitoring":"Resume monitoring"} className="rounded-md border border-[#DCE1E8] p-1.5 text-[#8A96A3] hover:text-[#1F2A37]">{d.enabled?<Pause size={14}/>:<Play size={14}/>}</button></div></td></tr>}/>):<tr><td colSpan={9} className="py-16 text-center"><div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-[#EEF1F4]"><Server size={22} className="text-[#8A96A3]"/></div><div className="font-semibold text-[#1F2A37]">No devices registered</div><p className="mt-1 text-xs text-[#8A96A3]">Start with Add device or discover your subnet.</p></td></tr>}</tbody></table></div></section>
  <div className="mt-4 flex items-center justify-between text-[11px] text-[#8A96A3]"><span>RoutingNMS inventory · SNMP v2c/v3</span><span>Backend-driven · changes are persisted in PostgreSQL</span></div>
  {showAdd&&<EngModal title="Add device" subtitle="Register the device — saving starts monitoring on the intervals below." onClose={()=>{setShowAdd(false);setCreatingGroup(false)}} footer={<>
    <EngButton type="button" onClick={()=>{setShowAdd(false);setCreatingGroup(false)}}>Cancel</EngButton>
    <EngButton type="submit" form="add-device-form" variant="primary" disabled={saving}>{saving&&<Loader2 size={14} className="animate-spin"/>}Register & monitor</EngButton>
  </>}>
    <form id="add-device-form" ref={addFormRef} onSubmit={add} className="space-y-4">
      <EngSection title="Connection">
        <EngField label="Device name" wide><EngInput required name="name" placeholder="Core-Router-01" list="known-device-names"/></EngField>
        <EngField label="IP address" error={addressError} wide>
          <EngInput required name="address" placeholder="192.168.88.17" list="known-device-addresses"
            error={!!addressError}
            onChange={e=>setAddressError(e.target.value && !isValidIPv4(e.target.value) ? "Enter a valid IPv4 address (e.g. 192.168.88.17)." : "")}/>
          <datalist id="known-device-addresses">{devices.map(d=><option key={d.id} value={d.address}/>)}</datalist>
          <datalist id="known-device-names">{devices.map(d=><option key={d.id} value={d.name}/>)}</datalist>
        </EngField>
        <EngField label="Group" wide>
          <EngSelect name="groupSelect" onChange={e=>setCreatingGroup(e.target.value==="__new__")} defaultValue="">
            <option value="">Ungrouped</option>
            {groups.map(g=><option key={g.id} value={g.id}>{g.name}</option>)}
            <option value="__new__">+ Create new group…</option>
          </EngSelect>
          {creatingGroup&&<EngInput name="newGroupName" required placeholder="New group name" className="mt-2"/>}
        </EngField>
      </EngSection>
      <EngSection title="Monitoring">
        <div className="sm:col-span-2 rounded-[4px] border border-[#DCE1E8] p-3">
          <EngToggle name="icmpEnabled" defaultChecked label="Ping this device (ICMP)" description="Starts checking on the interval below as soon as it's saved."/>
        </div>
        <EngField label="Check interval (seconds)"><EngInput name="icmpIntervalSeconds" type="number" defaultValue="30" min="5"/></EngField>
        <div className="sm:col-span-2 mt-1 rounded-[4px] border border-[#DCE1E8] p-3">
          <EngToggle name="snmpEnabled" label="Enable SNMP monitoring" description="Poll this device for interface/system data."/>
        </div>
        <EngField label="Version"><EngSelect name="snmpVersion" defaultValue="2c"><option value="2c">SNMP v2c</option><option value="3">SNMP v3</option></EngSelect></EngField>
        <EngField label="Port"><EngInput name="snmpPort" type="number" defaultValue="161"/></EngField>
        <EngField label="Community string" wide><EngInput name="community" type="password" placeholder="public"/></EngField>
        <div className="sm:col-span-2 mt-1 rounded-[4px] border border-[#DCE1E8] p-3">
          <EngToggle name="portCheckEnabled" checked={portCheckOn} onChange={setPortCheckOn} label="Port/Service Check" description="TCP connect test, or a real HTTP(S) request with status-code validation."/>
        </div>
        {portCheckOn && <>
          <EngField label="Protocol"><EngSelect name="portCheckProtocol" value={portCheckProtocol} onChange={e=>setPortCheckProtocol(e.target.value)}><option value="tcp">TCP</option><option value="http">HTTP</option><option value="https">HTTPS</option></EngSelect></EngField>
          <EngField label="Port"><EngInput name="portCheckPort" type="number" placeholder={portCheckProtocol==="https"?"443":portCheckProtocol==="http"?"80":"e.g. 22"} defaultValue={portCheckProtocol==="https"?"443":portCheckProtocol==="http"?"80":""}/></EngField>
          {portCheckProtocol!=="tcp"&&<>
            <EngField label="Path"><EngInput name="portCheckPath" defaultValue="/" placeholder="/"/></EngField>
            <EngField label="Expected status code"><EngInput name="portCheckAcceptedStatusCodes" defaultValue="200-299" placeholder="200-299"/></EngField>
          </>}
          <EngField label="Check interval (seconds)" wide={portCheckProtocol==="tcp"}><EngInput name="portCheckIntervalSeconds" type="number" defaultValue="60" min="5"/></EngField>
        </>}
      </EngSection>
    </form>
  </EngModal>}
  {showTest&&<Modal title="Verify SNMP connectivity" subtitle="Test credentials before adding a device to monitoring." onClose={()=>setShowTest(false)}><form onSubmit={test} className="space-y-4"><div className="grid gap-4 sm:grid-cols-2"><Field label="Device name"><input required name="name" placeholder="Core-Router-01" className={input}/></Field><Field label="IP address"><input required name="address" placeholder="192.168.88.17" className={input}/></Field><Field label="Type"><select name="deviceType" className={input}><option value="router">Router</option><option value="switch">Switch</option><option value="olt">OLT</option></select></Field><Field label="Vendor"><input name="vendor" placeholder="MikroTik" className={input}/></Field><Field label="Version"><select name="version" className={input}><option value="2c">SNMP v2c</option><option value="3">SNMP v3</option></select></Field><Field label="Community"><input required name="community" type="password" placeholder="public" className={input}/></Field><Field label="Port"><input name="snmpPort" type="number" defaultValue="161" className={input}/></Field><Field label="Timeout"><input name="timeoutMs" type="number" defaultValue="3000" className={input}/></Field></div><button disabled={testing} className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-[#BFD7FB] bg-[#EEF3FD] px-4 py-3 text-sm font-bold text-[#1D4ED8]">{testing&&<Loader2 size={16} className="animate-spin"/>}{testing?"Testing SNMP…":"Test connection & discover interfaces"}</button>{result&&<div className={`rounded-lg border p-4 ${result.reachable?"border-[#BFE3D0] bg-[#E9F5EF]":"border-[#F0C2C2] bg-[#FBEAEA]"}`}><div className="flex items-center justify-between gap-3"><b className={result.reachable?"text-[#1E8E5A]":"text-[#C4362D]"}>{result.reachable?"SNMP reachable":"SNMP test failed"}</b>{result.interfaceCount!==undefined&&<span className="text-xs text-[#1D4ED8]">{result.interfaceCount} interfaces</span>}</div>{result.systemName&&<p className="mt-2 text-sm text-[#5C6B7A]">{result.systemName}</p>}{result.sysDescr&&<p className="mt-1 text-xs text-[#8A96A3]">{result.sysDescr}</p>}{result.error&&<pre className="mt-2 whitespace-pre-wrap text-xs text-[#C4362D]">{result.error}</pre>}</div>}</form></Modal>}
  {selected&&<Modal title={`SNMP · ${selected.name}`} subtitle={`${selected.address} · ${selected.vendor||selected.deviceType}`} onClose={()=>setSelected(null)}><form onSubmit={saveSNMP} className="space-y-5"><div className="flex items-center justify-between rounded-lg border border-[#DCE1E8] bg-[#F9FAFC] p-4"><div><b className="text-sm text-[#1F2A37]">Enable SNMP monitoring</b><p className="mt-1 text-xs text-[#8A96A3]">Poll this device and make interface data available in Monitoring.</p></div><input name="enabled" type="checkbox" defaultChecked={selected.snmpEnabled} className="h-5 w-5"/></div><div className="grid gap-4 sm:grid-cols-2"><Field label="Version"><select name="version" defaultValue={selected.snmpVersion||"2c"} className={input}><option value="2c">SNMP v2c</option><option value="3">SNMP v3</option></select></Field><Field label="Port"><input name="port" type="number" defaultValue={selected.snmpPort||161} className={input}/></Field><Field label="Community"><input name="community" type="password" placeholder="Leave blank to keep current" className={input}/></Field><Field label="Username"><input name="username" placeholder="SNMPv3 username" className={input}/></Field><Field label="Auth protocol"><select name="authProto" className={input}><option value="">None</option><option value="MD5">MD5</option><option value="SHA">SHA</option></select></Field><Field label="Auth password"><input name="authPass" type="password" className={input}/></Field><Field label="Privacy protocol"><select name="privProto" className={input}><option value="">None</option><option value="DES">DES</option><option value="AES">AES</option></select></Field><Field label="Privacy password"><input name="privPass" type="password" className={input}/></Field><Field label="Timeout"><input name="timeoutMs" type="number" defaultValue="3000" className={input}/></Field></div><div className="flex justify-end gap-2 border-t border-[#DCE1E8] pt-4"><button type="button" onClick={()=>setSelected(null)} className="rounded-lg border border-[#DCE1E8] px-4 py-2.5 text-sm text-[#5C6B7A]">Cancel</button><button disabled={saving} className="rounded-lg bg-[#2E7BF6] px-5 py-2.5 text-sm font-medium text-white disabled:opacity-50">{saving?"Saving…":"Save SNMP"}</button></div></form></Modal>}
 </main>
}
function Stat({icon:Icon,label,value,hint,good,warn}:{icon:React.ComponentType<{size?:number}>;label:string;value:number;hint:string;good?:boolean;warn?:boolean}){return <div className={`${card} p-4`}><div className="flex items-start justify-between"><div><div className="text-[11px] uppercase tracking-wider text-[#8A96A3]">{label}</div><div className="mt-2 text-2xl font-bold text-[#1F2A37]">{value}</div><div className={`mt-1 text-[11px] ${warn?"text-[#C77700]":good?"text-[#1E8E5A]":"text-[#8A96A3]"}`}>{hint}</div></div><div className={`rounded-lg p-2 ${warn?"bg-[#FDF3E4] text-[#C77700]":good?"bg-[#E9F5EF] text-[#1E8E5A]":"bg-[#EEF1F4] text-[#5C6B7A]"}`}><Icon size={18}/></div></div></div>}
function DeviceIcon({type}:{type:string}){const Icon=type==="router"?Router:type==="switch"?Network:type==="olt"?Wifi:Server;return <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-[#EEF1F4] text-[#1D4ED8]"><Icon size={17}/></span>}
function Field({label,children,wide}:{label:string;children:React.ReactNode;wide?:boolean}){return <label className={`block text-xs font-medium text-[#5C6B7A] ${wide?"sm:col-span-2":""}`}>{label}{children}</label>}
function Step({n,title,children}:{n:string;title:string;children:React.ReactNode}){return <section className="rounded-lg border border-[#DCE1E8] bg-[#F9FAFC] p-4"><div className="mb-4 flex items-center gap-3"><span className="font-mono text-[10px] text-[#2E7BF6]">{n}</span><h3 className="text-sm font-semibold text-[#1F2A37]">{title}</h3></div>{children}</section>}
function Modal({title,subtitle,onClose,children}:{title:string;subtitle:string;onClose:()=>void;children:React.ReactNode}){return <div className="fixed inset-0 z-50 flex items-center justify-center bg-[#1F2A37]/50 p-4 backdrop-blur-sm"><div className="max-h-[92vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-[#DCE1E8] bg-white shadow-2xl"><div className="sticky top-0 z-10 flex items-start justify-between border-b border-[#DCE1E8] bg-white/95 px-6 py-5 backdrop-blur"><div><h2 className="text-lg font-bold text-[#1F2A37]">{title}</h2><p className="mt-1 text-xs text-[#8A96A3]">{subtitle}</p></div><button onClick={onClose} className="rounded-md p-1.5 text-[#8A96A3] hover:bg-[#EEF1F4] hover:text-[#1F2A37]"><X size={18}/></button></div><div className="p-6">{children}</div></div></div>}
