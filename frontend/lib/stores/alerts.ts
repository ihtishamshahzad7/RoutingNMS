"use client";

// Global live-alert store (Zustand). Centralizes the alert feed so the top
// bar badge, dashboard feed and any NOC surface can share one source of
// truth without per-page duplication. This is additive - existing pages can
// keep polling their own endpoints; this store powers the new alert feed /
// badge surfaces.
import { create } from "zustand";

export type LiveAlert = {
  id: string;
  severity: "critical" | "warning" | "info" | string;
  deviceName: string;
  message: string;
  minutesAgo: number;
  site?: string;
};

type AlertState = {
  alerts: LiveAlert[];
  setAlerts: (a: LiveAlert[]) => void;
  addAlert: (a: LiveAlert) => void;
};

export const useAlertStore = create<AlertState>((set) => ({
  alerts: [],
  setAlerts: (alerts) => set({ alerts }),
  addAlert: (a) =>
    set((state) => ({
      // keep newest first, dedupe by id, cap the feed at 50
      alerts: [a, ...state.alerts.filter((x) => x.id !== a.id)].slice(0, 50),
    })),
}));
