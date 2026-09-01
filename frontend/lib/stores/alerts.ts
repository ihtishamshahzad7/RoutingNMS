"use client";

// Global live-alert store (Zustand). Centralizes the active-alert feed so the
// top bar badge, dashboard feed and any NOC surface can share one source of
// truth without per-page duplication. Populated by polling the backend's
// GET /api/v1/alerts/active (see top-bar.tsx / useActiveAlerts).
import { create } from "zustand";

export type ActiveAlert = {
  id: string;
  source: "olt" | "device" | "trap" | string;
  severity: "critical" | "warning" | "info" | string;
  hostname: string;
  message: string;
  since: string;
};

type AlertState = {
  alerts: ActiveAlert[];
  loadedAt: number | null;
  setAlerts: (a: ActiveAlert[]) => void;
  clear: () => void;
};

export const useAlertStore = create<AlertState>((set) => ({
  alerts: [],
  loadedAt: null,
  setAlerts: (alerts) => set({ alerts, loadedAt: Date.now() }),
  clear: () => set({ alerts: [], loadedAt: null }),
}));