"use client";

// Feature 0.5 (Design System): a living style-guide page, not a markdown
// doc alone -- it renders the actual design tokens and the actual
// components/ui/* primitives, so it breaks (visibly, in the browser) the
// moment a token or component regresses, which is the same
// "real-hardware-testable Definition of Done" standard this build
// blueprint already asks of backend features. Internal engineering
// reference, not a NOC monitoring page -- lives under the "Account" nav
// group next to Settings rather than Monitoring/Network/etc.
import { useState } from "react";
import { Button, Tag, AiBadge, AlertBadge } from "../../../components/ui/primitives";
import { Card, StatCard } from "../../../components/ui/card";
import { StatusDot } from "../../../components/ui/status-dot";
import { StatusPill } from "../../../components/ui/status-pill";
import { Input, Select, Textarea, FieldLabel, Checkbox, PageHeader, Banner, Panel } from "../../../components/ui/form";
import { Modal } from "../../../components/ui/modal";
import { Tooltip } from "../../../components/ui/tooltip";
import { Sparkline } from "../../../components/ui/sparkline";

const TOKENS: { name: string; cssVar: string; swatch: string }[] = [
  { name: "bg-page", cssVar: "--color-bg-page", swatch: "#0d1117" },
  { name: "bg-surface", cssVar: "--color-bg-surface", swatch: "#161b22" },
  { name: "bg-raised", cssVar: "--color-bg-raised", swatch: "#1c2128" },
  { name: "border", cssVar: "--color-border", swatch: "#21262d" },
  { name: "border-strong", cssVar: "--color-border-strong", swatch: "#30363d" },
  { name: "text-primary", cssVar: "--color-text-primary", swatch: "#e6edf3" },
  { name: "text-muted", cssVar: "--color-text-muted", swatch: "#8b949e" },
  { name: "text-ghost", cssVar: "--color-text-ghost", swatch: "#484f58" },
  { name: "status-up", cssVar: "--color-status-up", swatch: "#3fb950" },
  { name: "status-warn", cssVar: "--color-status-warn", swatch: "#d29922" },
  { name: "status-crit", cssVar: "--color-status-crit", swatch: "#f78166" },
  { name: "status-info", cssVar: "--color-status-info", swatch: "#58a6ff" },
  { name: "status-ai", cssVar: "--color-status-ai", swatch: "#a371f7" },
];

const STATUSES = ["up", "warning", "critical", "unknown", "info", "analyzing"];

function Section({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <Card title={title} className="mb-6">
      <div className="p-4">
        {description && <p className="mb-4 text-xs text-text-muted">{description}</p>}
        {children}
      </div>
    </Card>
  );
}

export default function StyleGuidePage() {
  const [showModal, setShowModal] = useState(false);

  return (
    <div>
      <PageHeader
        eyebrow="Design system"
        title="Style Guide"
        description="Feature 0.5 of the build blueprint: a living reference for RoutingNMS's GitHub-dark design system (tokens defined in app/globals.css, components in components/ui/). Check here before introducing a new color, spacing value, or a hand-rolled version of something already below."
      />

      <Section title="Color tokens" description="Every @theme token in app/globals.css. Also available as Tailwind utilities (bg-<name>, text-<name>, border-<name>).">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
          {TOKENS.map((t) => (
            <div key={t.name} className="flex items-center gap-3 rounded-[6px] border border-border p-2">
              <span
                className="h-8 w-8 shrink-0 rounded-[5px] border border-border-strong"
                style={{ background: t.swatch }}
              />
              <div className="min-w-0">
                <div className="truncate text-[11px] font-semibold text-text-primary">{t.name}</div>
                <div className="truncate font-mono text-[10px] text-text-ghost">{t.cssVar}</div>
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Buttons" description="components/ui/primitives.tsx — Button">
        <div className="flex flex-wrap gap-3">
          <Button variant="primary">Primary</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="danger">Danger</Button>
          <Button variant="primary" disabled>
            Disabled
          </Button>
        </div>
      </Section>

      <Section title="Badges & tags" description="components/ui/primitives.tsx — AlertBadge, Tag, AiBadge">
        <div className="flex flex-wrap items-center gap-3">
          <AlertBadge count={3} />
          <Tag>snmp</Tag>
          <Tag>core-router</Tag>
          <AiBadge />
        </div>
      </Section>

      <Section title="Status dots & pills" description="components/ui/status-dot.tsx, status-pill.tsx — colors now sourced from the shared components/ui/statusColors.ts table.">
        <div className="mb-4 flex flex-wrap items-center gap-4">
          {STATUSES.map((s) => (
            <div key={s} className="flex items-center gap-2">
              <StatusDot status={s} pulse={s === "critical"} />
              <span className="text-[11px] text-text-muted">{s}</span>
            </div>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {STATUSES.map((s) => (
            <StatusPill key={s} status={s} pulse={s === "critical"} />
          ))}
        </div>
      </Section>

      <Section title="Cards & stat cards" description="components/ui/card.tsx — Card, StatCard">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="Total Nodes" value={128} />
          <StatCard label="Active Alerts" value={3} accent="text-status-crit" />
          <StatCard label="Global Latency" value={24} unit="ms" />
          <StatCard label="Uptime" value="99.98" unit="%" accent="text-status-up" />
        </div>
      </Section>

      <Section title="Sparkline" description="components/ui/sparkline.tsx">
        <div className="w-48">
          <Sparkline points={[3, 5, 4, 8, 6, 9, 7, 12, 10, 14]} />
        </div>
      </Section>

      <Section title="Form fields" description="components/ui/form.tsx — Input, Select, Textarea, FieldLabel, Checkbox, Banner, Panel">
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <FieldLabel>Device name</FieldLabel>
            <Input placeholder="Core-Router-01" />
          </div>
          <div>
            <FieldLabel>Device type</FieldLabel>
            <Select>
              <option>Router</option>
              <option>Switch</option>
              <option>OLT</option>
            </Select>
          </div>
          <div className="sm:col-span-2">
            <FieldLabel>Notes</FieldLabel>
            <Textarea rows={2} placeholder="Optional" />
          </div>
          <label className="flex items-center gap-2 text-xs text-text-muted">
            <Checkbox defaultChecked /> Enable SNMP monitoring
          </label>
        </div>
        <div className="mt-4 space-y-3">
          <Banner tone="info">Informational banner — matches the design system's info token.</Banner>
          <Banner tone="error">Error banner — matches the design system's critical token.</Banner>
          <Panel>A dark inner panel used to group related fields.</Panel>
        </div>
      </Section>

      <Section title="Modal" description="components/ui/modal.tsx — new in this feature; a shared, token-based dialog primitive for future pages to reuse instead of hand-rolling another one-off.">
        <Button variant="secondary" onClick={() => setShowModal(true)}>
          Open example modal
        </Button>
        {showModal && (
          <Modal title="Example modal" subtitle="Built on the same tokens as everything else on this page." onClose={() => setShowModal(false)}>
            <p className="text-sm text-text-muted">Modal body content goes here.</p>
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="secondary" onClick={() => setShowModal(false)}>
                Cancel
              </Button>
              <Button variant="primary" onClick={() => setShowModal(false)}>
                Confirm
              </Button>
            </div>
          </Modal>
        )}
      </Section>

      <Section title="Tooltip" description="components/ui/tooltip.tsx — new in this feature.">
        <Tooltip label="This is a tooltip">
          <Button variant="secondary">Hover me</Button>
        </Tooltip>
      </Section>
    </div>
  );
}
