"use client";

// Device groups: a lightweight, named-folder concept for organizing devices,
// ported from Uptime Kuma's per-status-page monitor grouping. Distinct from
// tags (free-form, cross-cutting labels) -- a group is an exclusive-ish
// folder, assigned from a device's row action on the Devices page.

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, FieldLabel } from "../../../components/ui/form";

type Group = { id: number; name: string; sortOrder: number };

const ORG = "tenant-1";

export default function DeviceGroupsAdmin() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Group | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      setGroups(await apiFetch<Group[]>(`/device-groups?tenantId=${ORG}`));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load device groups.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    setSaving(true);
    setMessage("");
    const data = new FormData(e.currentTarget);
    const payload = { tenantId: ORG, name: String(data.get("name") || "").trim(), sortOrder: Number(data.get("sortOrder") || 0) };
    try {
      if (editing.id) {
        await apiFetch(`/device-groups/${editing.id}`, { method: "PUT", body: JSON.stringify(payload) });
      } else {
        await apiFetch("/device-groups", { method: "POST", body: JSON.stringify(payload) });
      }
      setMessage(`Group "${payload.name}" saved.`);
      setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save group.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(g: Group) {
    try {
      await apiFetch(`/device-groups/${g.id}`, { method: "DELETE" });
      setMessage(`Group "${g.name}" deleted. Its devices are now ungrouped.`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete group.");
    }
  }

  return (
    <main className="mx-auto max-w-4xl px-6 py-6">
      <PageHeader
        eyebrow="Organize"
        title="Device Groups"
        description="Named, collapsible folders for organizing the Devices list and Reachability board — assign a device to a group from its row action. A device belongs to at most one group; unassigned devices show under Ungrouped."
      />
      {message && <Banner>{message}</Banner>}

      {!editing ? (
        <Card
          title={`All groups (${groups.length})`}
          headerRight={<Button variant="primary" onClick={() => setEditing({ id: 0, name: "", sortOrder: groups.length })}>New group</Button>}
          className="p-4"
        >
          <div className="flex flex-col gap-2">
            {loading ? (
              <div className="w-full py-8 text-center text-sm text-[#484F58]">Loading…</div>
            ) : groups.length ? (
              groups.map(g => (
                <div key={g.id} className="flex items-center justify-between rounded-[6px] border border-[#21262D] bg-[#0D1117] px-3 py-2 text-sm">
                  <span className="text-[#E6EDF3]">{g.name}</span>
                  <div className="flex items-center gap-3">
                    <span className="text-[10px] text-[#484F58]">order {g.sortOrder}</span>
                    <button onClick={() => setEditing(g)} className="text-xs text-[#58A6FF] hover:underline">Rename</button>
                    <button onClick={() => remove(g)} className="text-xs text-[#F78166] hover:underline">Delete</button>
                  </div>
                </div>
              ))
            ) : (
              <div className="w-full py-8 text-center text-sm text-[#484F58]">No device groups yet.</div>
            )}
          </div>
        </Card>
      ) : (
        <Card
          title={editing.id ? `Edit "${editing.name}"` : "New group"}
          headerRight={<button onClick={() => setEditing(null)} className="text-[#8B949E] hover:text-[#E6EDF3]">✕</button>}
          className="p-4"
        >
          <form onSubmit={save} className="grid gap-4">
            <FieldLabel>Name<Input required name="name" defaultValue={editing.name} placeholder="Core Network, Customer Edge, Datacenter A…" /></FieldLabel>
            <FieldLabel>Sort order<Input name="sortOrder" type="number" defaultValue={editing.sortOrder} /></FieldLabel>
            <div className="flex gap-3">
              <Button type="button" onClick={() => setEditing(null)} className="flex-1 justify-center">Cancel</Button>
              <Button variant="primary" disabled={saving} className="flex-1 justify-center">{saving ? "Saving…" : "Save group"}</Button>
            </div>
          </form>
        </Card>
      )}
    </main>
  );
}
