"use client";

import { useEffect } from "react";
import { useWorkspaceStore } from "./store";
import type { LiveReading } from "./types";

// Subscribes to the backend's per-group SSE push (GET
// /api/v1/workspace-topology/groups/{id}/stream, backed by
// workspacetopology.LiveHub) and feeds every batch into the store via
// applyLiveReadings. Mirrors the app's existing SSE pattern
// (components/incident-live.tsx's `new EventSource(...)` +
// addEventListener) rather than introducing a new realtime mechanism --
// this codebase deliberately uses SSE, not WebSockets, for all realtime
// (see AGENTS.md).
//
// Reconnects automatically: EventSource itself retries on a dropped
// connection, but the browser's default retry delay is unspecified/slow,
// so this also runs its own bounded backoff loop when a stream closes
// with an error, up to a 15s ceiling.
export function useLiveStream(groupId: string | null) {
  const applyLiveReadings = useWorkspaceStore((s) => s.applyLiveReadings);
  const setLiveStreamConnected = useWorkspaceStore((s) => s.setLiveStreamConnected);

  useEffect(() => {
    if (!groupId) {
      setLiveStreamConnected(false);
      return;
    }

    let cancelled = false;
    let source: EventSource | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;
    let retryDelay = 1000;

    function connect() {
      if (cancelled) return;
      source = new EventSource(`/api/v1/workspace-topology/groups/${groupId}/stream`);

      source.addEventListener("readings", (e) => {
        retryDelay = 1000; // reset backoff on any successful message
        setLiveStreamConnected(true);
        try {
          const readings = JSON.parse((e as MessageEvent).data) as LiveReading[];
          applyLiveReadings(readings);
        } catch {
          // malformed event -- ignore this one, the stream keeps going
        }
      });

      source.onerror = () => {
        setLiveStreamConnected(false);
        source?.close();
        if (cancelled) return;
        retryTimer = setTimeout(() => {
          retryDelay = Math.min(retryDelay * 2, 15000);
          connect();
        }, retryDelay);
      };
    }

    connect();

    return () => {
      cancelled = true;
      setLiveStreamConnected(false);
      if (retryTimer) clearTimeout(retryTimer);
      source?.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupId]);
}
