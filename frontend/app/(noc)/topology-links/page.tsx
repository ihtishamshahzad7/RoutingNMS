"use client";

// Topology Links -- group-wise, port-level topology mapping: an operator
// explicitly states "device A interface ethX is connected to device B
// interface ethY", organized into named groups. Distinct from the
// auto-generated LLDP topology at /topology (a discovered graph with no
// concept of a group or an operator-defined link) -- this is the manual
// complement Zabbix-style NMS operators expect, with real-time SNMP
// ifOperStatus polling and alerting on both ends.

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { Input, PageHeader, Banner, Panel, FieldLabel } from "../../../components/ui/form";
import { StatusPill } from "../../../components/ui/status-pill";

const ORG = "tenant-1";

type Group = { id: string; organizationId: string; name: string; createdAt: string };
type Link = {
  id: string; groupId: string;
  deviceAId: string; deviceAName?: string; interfaceA: string;
  deviceBId: string; deviceBName?: string; interfaceB: string;
  createdAt: string;
};
type Device = { id: string; name: string; address: string };
type LinkStatus = { linkId: string; up: boolean; sideAUp?: boolean | null; sideBUp?: boolean | null; error?: string; checkedAt: string };

export default function TopologyLinksPage() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [selectedGroup, setSelectedGroup] = useState<string>("");
  const [links, setLinks] = useState<Link[]>([]);
  const [statuses, setStatuses] = useState<Record<string, LinkStatus>>({});
  const [devices, setDevices] = useState<Device[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [creatingGroup, setCreatingGroup] = useState(false);
  const [creatingLink, setCreatingLink] = useState(false);

  async function loadGroups() {
    try {
      const g = await apiFetch<Group[]>(`/topology-groups?organizationId=${ORG}`);
      setGroups(g);
      if (!selectedGroup && g.length > 0) setSelectedGroup(g[0].id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load topology groups.");
    }
  }

  async function loadLinks(groupId: string) {
    if (!groupId) { setLinks([]); return; }
    try {
      setLinks(await apiFetch<Link[]>(`/topology-groups/${groupId}/links`));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load topology links.");
    }
  }

  async function loadStatus(groupId: string) {
    if (!groupId) return;
    try {
      setStatuses(await apiFetch<Record<string, LinkStatus>>(`/topology-groups/${groupId}/status`));
    } catch { /* best-effort; keep last known statuses */ }
  }

  useEffect(() => { loadGroups(); }, []);
  useEffect(() => { apiFetch<Device[]>(`/devices?organizationId=${ORG}`).then(setDevices).catch(() => {}); }, []);
  useEffect(() => { loadLinks(selectedGroup); loadStatus(selectedGroup); }, [selectedGroup]);
  useEffect(() => {
    if (!selectedGroup) return;
    const t = setInterval(() => loadStatus(selectedGroup), 10000);
    return () => clearInterval(t);
  }, [selectedGroup]);

  async function createGroup(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setCreatingGroup(true); setError("");
    const data = new FormData(e.currentTarget);
    try {
      const g = await apiFetch<Group>("/topology-groups", {
        method: "POST",
        body: JSON.stringify({ organizationId: ORG, name: data.get("name") }),
      });
      setGroups((prev) => [...prev, g]);
      setSelectedGroup(g.id);
      e.currentTarget.reset();
      setMessage(`Created group "${g.name}".`);
    } catch (e2) {
      setError(e2 instanceof ApiError ? e2.message : "Failed to create group.");
    } finally {
      setCreatingGroup(false);
    }
  }

  async function deleteGroup(id: string) {
    try {
      await apiFetch(`/topology-groups/${id}`, { method: "DELETE" });
      setGroups((prev) => prev.filter((g) => g.id !== id));
      if (selectedGroup === id) setSelectedGroup("");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to delete group.");
    }
  }

  async function createLink(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!selectedGroup) return;
    setCreatingLink(true); setError("");
    const data = new FormData(e.currentTarget);
    try {
      const link = await apiFetch<Link>(`/topology-groups/${selectedGroup}/links`, {
        method: "POST",
        body: JSON.stringify({
          deviceAId: data.get("deviceAId"),
          interfaceA: data.get("interfaceA"),
          deviceBId: data.get("deviceBId"),
          interfaceB: data.get("interfaceB"),
        }),
      });
      setLinks((prev) => [...prev, link]);
      e.currentTarget.reset();
      setMessage("Link created — SNMP polling will confirm status on the next poll cycle.");
      loadStatus(selectedGroup);
    } catch (e2) {
      setError(e2 instanceof ApiError ? e2.message : "Failed to create link.");
    } finally {
      setCreatingLink(false);
    }
  }

  async function deleteLink(id: string) {
    try {
      await apiFetch(`/topology-links/${id}`, { method: "DELETE" });
      setLinks((prev) => prev.filter((l) => l.id !== id));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to delete link.");
    }
  }

  function statusFor(linkId: string): LinkStatus | undefined {
    return statuses[linkId];
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-6">
      <PageHeader
        eyebrow="NETWORK / TOPOLOGY"
        title="Topology Links"
        description="Manually map device-to-device port connections, organized into groups (e.g. 'Group 1'). RoutingNMS polls each end's SNMP ifOperStatus in real time and alerts by name — group, device and port — through the same alert pipeline as every other monitor. Distinct from the auto-generated LLDP graph on the Topology page."
      />
      {message && <Banner>{message}</Banner>}
      {error && <Banner tone="error">{error}</Banner>}

      <div className="grid gap-5 lg:grid-cols-[280px_1fr]">
        <Card title="Groups" className="p-4">
          <form onSubmit={createGroup} className="mb-4 flex gap-2">
            <Input name="name" placeholder="e.g. Group 1" required className="mt-0" />
            <Button type="submit" variant="primary" disabled={creatingGroup}>{creatingGroup ? "…" : "Add"}</Button>
          </form>
          <div className="flex flex-col gap-1">
            {groups.length === 0 && <div className="text-xs text-[#8B949E]">No groups yet — create one above.</div>}
            {groups.map((g) => (
              <div key={g.id} className={`flex items-center justify-between rounded-[6px] px-3 py-2 text-sm ${selectedGroup === g.id ? "bg-[#1C2128] text-[#E6EDF3]" : "text-[#8B949E] hover:bg-[#1C2128]/60"}`}>
                <button className="flex-1 text-left" onClick={() => setSelectedGroup(g.id)}>{g.name}</button>
                <button className="ml-2 text-xs text-[#F78166] hover:underline" onClick={() => deleteGroup(g.id)}>delete</button>
              </div>
            ))}
          </div>
        </Card>

        <div className="flex flex-col gap-5">
          <Card title="Add a link" className="p-4">
            {!selectedGroup ? (
              <div className="text-xs text-[#8B949E]">Select or create a group first.</div>
            ) : (
              <form onSubmit={createLink} className="grid gap-4 sm:grid-cols-2">
                <FieldLabel>Device A
                  <select name="deviceAId" required className="mt-1 w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm text-[#E6EDF3] outline-none focus:border-[#58A6FF]">
                    <option value="">Select device…</option>
                    {devices.map((d) => <option key={d.id} value={d.id}>{d.name} ({d.address})</option>)}
                  </select>
                </FieldLabel>
                <FieldLabel>Interface A<Input name="interfaceA" placeholder="e.g. eth1" required /></FieldLabel>
                <FieldLabel>Device B
                  <select name="deviceBId" required className="mt-1 w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm text-[#E6EDF3] outline-none focus:border-[#58A6FF]">
                    <option value="">Select device…</option>
                    {devices.map((d) => <option key={d.id} value={d.id}>{d.name} ({d.address})</option>)}
                  </select>
                </FieldLabel>
                <FieldLabel>Interface B<Input name="interfaceB" placeholder="e.g. eth1" required /></FieldLabel>
                <div className="sm:col-span-2">
                  <Button type="submit" variant="primary" disabled={creatingLink}>{creatingLink ? "Creating…" : "Create link"}</Button>
                </div>
              </form>
            )}
          </Card>

          <Card title={`Links${selectedGroup ? "" : " (no group selected)"}`} className="p-4">
            {links.length === 0 ? (
              <div className="text-xs text-[#8B949E]">No links in this group yet.</div>
            ) : (
              <div className="flex flex-col gap-3">
                {links.map((l) => {
                  const st = statusFor(l.id);
                  return (
                    <Panel key={l.id} className="flex flex-wrap items-center justify-between gap-3">
                      <div className="text-sm text-[#E6EDF3]">
                        <span className="font-semibold">{l.deviceAName || l.deviceAId}</span>
                        <span className="text-[#8B949E]"> · {l.interfaceA} </span>
                        <span className="text-[#58A6FF]">↔</span>
                        <span className="text-[#8B949E]"> {l.interfaceB} · </span>
                        <span className="font-semibold">{l.deviceBName || l.deviceBId}</span>
                      </div>
                      <div className="flex items-center gap-3">
                        {!st ? (
                          <StatusPill status="unknown" label="No data yet" />
                        ) : (
                          <>
                            <StatusPill status={st.sideAUp ? "up" : "down"} label={`A: ${st.sideAUp ? "up" : "down"}`} />
                            <StatusPill status={st.sideBUp ? "up" : "down"} label={`B: ${st.sideBUp ? "up" : "down"}`} />
                            <StatusPill status={st.up ? "up" : "critical"} label={st.up ? "Link up" : "Link down"} />
                          </>
                        )}
                        <button className="text-xs text-[#F78166] hover:underline" onClick={() => deleteLink(l.id)}>delete</button>
                      </div>
                      {st?.error && <div className="w-full text-xs text-[#D29922]">{st.error}</div>}
                    </Panel>
                  );
                })}
              </div>
            )}
          </Card>
        </div>
      </div>
    </main>
  );
}
