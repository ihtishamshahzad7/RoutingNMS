// Shared fetch helper for talking to the RoutingNMS backend.
//
// Deliberately relative ("/api/v1/...", not "http://127.0.0.1:8080/...").
// The browser resolves a relative URL against the page's own origin, so a
// request goes to http://<server-ip>/api/v1/... and Nginx's `location
// /api/` block proxies it to the Go API on 127.0.0.1:8080. Hardcoding
// 127.0.0.1:8080 here would instead make every visitor's browser try to
// reach port 8080 on their *own* machine, which only happens to work when
// testing from the server itself.
const API_BASE = "/api/v1";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

/**
 * Calls a RoutingNMS API endpoint and returns the parsed JSON body. Always
 * sends the session cookie (`credentials: "include"`) so authenticated
 * requests work the same from any page.
 *
 * `path` is relative to /api/v1 (e.g. "/alerts/active"), except legacy routes
 * that live directly under /api (e.g. "/api/olts/...", "/api/topology",
 * "/api/incidents") — pass those with the full "/api/..." prefix and they are
 * used as-is.
 *
 * Throws ApiError for a reachable-but-unsuccessful response (4xx/5xx), and a
 * plain Error (network failure) when the backend cannot be reached at all —
 * callers should handle these two cases separately (invalid/expired session
 * vs. "the API is offline").
 */
export async function apiFetch<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  const resolved = path.startsWith("/api/") ? path : `${API_BASE}${path}`;
  const response = await fetch(resolved, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    cache: "no-store",
  });

  let body: unknown = null;
  try {
    body = await response.json();
  } catch {
    // Non-JSON or empty body; leave body as null.
  }

  if (!response.ok) {
    const message =
      (body && typeof body === "object" && "error" in body && typeof (body as { error?: unknown }).error === "string"
        ? (body as { error: string }).error
        : undefined) ?? `Request failed with status ${response.status}`;
    throw new ApiError(response.status, message);
  }

  return body as T;
}
