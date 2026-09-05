"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, Textarea, FieldLabel } from "../../../components/ui/form";

type Template = { id: number; name: string; scriptBody: string; createdAt: string; updatedAt: string };

export default function ProvisioningPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Template | null>(null);

  async function load() {
    setLoading(true);
    try {
      setTemplates(await apiFetch<Template[]>("/provisioning/templates"));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load provisioning templates.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setMessage("");
    const data = new FormData(e.currentTarget);
    const body = JSON.stringify({ name: data.get("name"), scriptBody: data.get("scriptBody") });
    try {
      if (editing) {
        await apiFetch<Template>(`/provisioning/templates/${editing.id}`, { method: "PUT", body });
        setMessage(`Template "${data.get("name")}" updated.`);
      } else {
        await apiFetch<Template>("/provisioning/templates", { method: "POST", body });
        setMessage(`Template "${data.get("name")}" created.`);
      }
      setEditing(null);
      e.currentTarget.reset();
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save template.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(t: Template) {
    try {
      await apiFetch(`/provisioning/templates/${t.id}`, { method: "DELETE" });
      setMessage(`Template "${t.name}" deleted.`);
      if (editing?.id === t.id) setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete template.");
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-6">
      <PageHeader
        eyebrow="Automation"
        title="RouterOS Provisioning Templates"
        description={
          <>
            Templates are RouterOS scripts rendered per-device with <code>{"{{.Hostname}}"}</code>, <code>{"{{.Address}}"}</code> and <code>{"{{.Password}}"}</code>.
            Assign a template to a router device from its detail page — the router then pulls its config itself via <code>/tool fetch</code>.
            Admin must pre-register the router (with its serial number) first; there is no auto-registration of unknown devices.
          </>
        }
      />
      {message && <Banner>{message}</Banner>}

      <div className="grid gap-6 lg:grid-cols-2">
        <Card title="Templates" headerRight={<Button onClick={load}>Refresh</Button>} className="p-4">
          <div className="space-y-3">
            {loading ? (
              <div className="py-8 text-center text-sm text-[#484F58]">Loading…</div>
            ) : templates.length ? (
              templates.map(t => (
                <div key={t.id} className="rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4">
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-medium text-[#E6EDF3]">{t.name}</div>
                    <div className="flex gap-2">
                      <Button onClick={() => setEditing(t)}>Edit</Button>
                      <Button variant="danger" onClick={() => remove(t)}>Delete</Button>
                    </div>
                  </div>
                  <pre className="mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap text-xs text-[#8B949E]">{t.scriptBody}</pre>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-sm text-[#484F58]">No templates yet.</div>
            )}
          </div>
        </Card>

        <Card title={editing ? `Edit "${editing.name}"` : "New template"} className="p-4">
          <form onSubmit={save} className="grid gap-4">
            <FieldLabel>
              Name
              <Input required name="name" defaultValue={editing?.name} />
            </FieldLabel>
            <FieldLabel>
              Script body
              <Textarea required name="scriptBody" defaultValue={editing?.scriptBody} rows={12} className="font-mono" />
            </FieldLabel>
            <div className="flex gap-3">
              {editing && (
                <Button type="button" onClick={() => setEditing(null)} className="flex-1 justify-center">
                  Cancel
                </Button>
              )}
              <Button variant="primary" disabled={saving} className="flex-1 justify-center">
                {saving ? "Saving…" : editing ? "Save changes" : "Create template"}
              </Button>
            </div>
          </form>
        </Card>
      </div>
    </main>
  );
}
