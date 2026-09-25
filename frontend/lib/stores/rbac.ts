"use client";

// Phase 0.1 (RoutingNMS build blueprint) frontend half: a small store +
// hook backing GET /api/v1/auth/permissions (backend/internal/rbac). This
// is the foundation only -- it is not yet wired into every button across
// the app (that would mean touching ~26 existing pages in one pass, which
// the project's own one-feature-at-a-time discipline argues against doing
// in the same commit as the schema/middleware). A page opts in by calling
// useHasPermission itself; nothing currently does, so this ships inert
// until the next increment starts using it.
import { create } from "zustand";
import { apiFetch, ApiError } from "../api";

interface RbacState {
  roles: string[];
  permissions: string[];
  loaded: boolean;
  loading: boolean;
  load: () => Promise<void>;
}

export const useRbacStore = create<RbacState>((set, get) => ({
  roles: [],
  permissions: [],
  loaded: false,
  loading: false,
  load: async () => {
    if (get().loading || get().loaded) return;
    set({ loading: true });
    try {
      const data = await apiFetch<{ roles: string[]; permissions: string[] }>("/auth/permissions");
      set({ roles: data.roles ?? [], permissions: data.permissions ?? [], loaded: true, loading: false });
    } catch (e) {
      // Not authenticated yet, or the endpoint is unreachable -- leave
      // roles/permissions empty (fail closed: useHasPermission returns
      // false, never true, when the check itself couldn't run) rather than
      // throwing out of a hook.
      if (!(e instanceof ApiError)) {
        // Network failure: still mark loaded so callers don't spin forever
        // waiting for a load that already happened and failed.
      }
      set({ roles: [], permissions: [], loaded: true, loading: false });
    }
  },
}));

/** Fetches once per session (lazily, on first call) and returns whether the
 * current user holds `key`. Always false until the load resolves -- callers
 * that gate a destructive action on this should treat "false" as "don't
 * know yet or not permitted" and rely on the backend's own
 * RequirePermission check as the real enforcement, per this project's
 * existing "frontend hides, backend rejects" rule (see Feature 0.1's
 * frontend workflow). */
export function useHasPermission(key: string): boolean {
  const { permissions, loaded, load } = useRbacStore();
  if (!loaded) {
    // Fire-and-forget; React re-renders when the store updates.
    void load();
  }
  return permissions.includes(key);
}
