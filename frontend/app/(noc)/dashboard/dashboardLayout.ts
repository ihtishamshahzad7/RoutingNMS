"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

// Customizable widget layout for the NOC dashboard grid. Purely a
// client-side presentation preference (which modules show, what order,
// and an optional gradient tint per widget) -- it never touches what data
// is fetched or how it's computed, only how the existing real data is
// arranged on screen. Persisted to localStorage per browser/user, the same
// tradeoff already made for other client-only state in this app (e.g. the
// Workspace Topology canvas positions before that feature grew backend
// persistence). No backend endpoint yet; see the "not addressed here" note
// in the commit this shipped in if that's ever worth promoting to a
// per-account server-side preference.

export type DashboardWidgetId =
  | "infra-map"
  | "performance"
  | "device-status"
  | "incident-summary"
  | "event-log"
  | "alerts-by-source";

export const WIDGET_CATALOG: { id: DashboardWidgetId; title: string; span: string }[] = [
  { id: "infra-map", title: "Global Infrastructure Map", span: "dashLg" },
  { id: "performance", title: "Performance Metrics", span: "dashMd" },
  { id: "device-status", title: "Device Status & Availability", span: "dashLg" },
  { id: "incident-summary", title: "Incident Summary", span: "dashMd" },
  { id: "event-log", title: "Live Event Log", span: "dashMd" },
  { id: "alerts-by-source", title: "Alerts by OLT", span: "dashMd" },
];

export const GRADIENT_PRESETS: { label: string; css: string | null }[] = [
  { label: "Default", css: null },
  { label: "Cyan glow", css: "linear-gradient(135deg, #161B22 0%, #0E2A33 120%)" },
  { label: "Crimson", css: "linear-gradient(135deg, #161B22 0%, #2E1418 120%)" },
  { label: "Emerald", css: "linear-gradient(135deg, #161B22 0%, #0E2A1D 120%)" },
  { label: "Violet", css: "linear-gradient(135deg, #161B22 0%, #241A3A 120%)" },
  { label: "Amber", css: "linear-gradient(135deg, #161B22 0%, #332107 120%)" },
];

interface DashboardLayoutState {
  order: DashboardWidgetId[];
  hidden: DashboardWidgetId[];
  gradients: Partial<Record<DashboardWidgetId, string>>;
  removeWidget: (id: DashboardWidgetId) => void;
  addWidget: (id: DashboardWidgetId) => void;
  moveWidget: (id: DashboardWidgetId, direction: -1 | 1) => void;
  reorderByDrag: (draggedId: DashboardWidgetId, targetId: DashboardWidgetId) => void;
  setGradient: (id: DashboardWidgetId, css: string | null) => void;
  resetLayout: () => void;
}

const DEFAULT_ORDER = WIDGET_CATALOG.map((w) => w.id);

export const useDashboardLayout = create<DashboardLayoutState>()(
  persist(
    (set) => ({
      order: DEFAULT_ORDER,
      hidden: [],
      gradients: {},
      removeWidget: (id) => set((s) => ({ hidden: s.hidden.includes(id) ? s.hidden : [...s.hidden, id] })),
      addWidget: (id) => set((s) => ({ hidden: s.hidden.filter((h) => h !== id) })),
      moveWidget: (id, direction) =>
        set((s) => {
          const visible = s.order.filter((w) => !s.hidden.includes(w));
          const idx = visible.indexOf(id);
          const swapWith = visible[idx + direction];
          if (idx === -1 || !swapWith) return s;
          const next = [...s.order];
          const a = next.indexOf(id);
          const b = next.indexOf(swapWith);
          [next[a], next[b]] = [next[b], next[a]];
          return { order: next };
        }),
      reorderByDrag: (draggedId, targetId) =>
        set((s) => {
          if (draggedId === targetId) return s;
          const next = s.order.filter((w) => w !== draggedId);
          const targetIdx = next.indexOf(targetId);
          next.splice(targetIdx, 0, draggedId);
          return { order: next };
        }),
      setGradient: (id, css) =>
        set((s) => {
          const next = { ...s.gradients };
          if (css) next[id] = css;
          else delete next[id];
          return { gradients: next };
        }),
      resetLayout: () => set({ order: DEFAULT_ORDER, hidden: [], gradients: {} }),
    }),
    { name: "routingnms-dashboard-layout" }
  )
);
