"use client";

// Public, unauthenticated status page -- deliberately outside the (noc)
// route group so it gets none of the authenticated app's sidebar/session
// requirements. Ported from Uptime Kuma's flagship "status page" feature:
// something you can share with a customer who has no RoutingNMS login.

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";

type ItemStatus = {
  subjectType: "device" | "olt";
  subjectId: string;
  label: string;
  status: "up" | "down" | "degraded" | "unknown";
  certExpiryDays?: number;
  since?: string;
};
type PublicPage = {
  slug: string;
  title: string;
  description: string;
  footerText: string;
  showCertificateExpiry: boolean;
  items: ItemStatus[];
  generatedAt: string;
};

const STATUS_STYLE: Record<ItemStatus["status"], { label: string; dot: string; text: string }> = {
  up: { label: "Operational", dot: "bg-emerald-400", text: "text-emerald-400" },
  degraded: { label: "Degraded", dot: "bg-amber-400", text: "text-amber-400" },
  down: { label: "Down", dot: "bg-red-500", text: "text-red-400" },
  unknown: { label: "Unknown", dot: "bg-slate-500", text: "text-slate-400" },
};

export default function PublicStatusPage() {
  const params = useParams<{ slug: string }>();
  const [page, setPage] = useState<PublicPage | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    try {
      const res = await fetch(`/api/v1/public/status/${params.slug}`, { cache: "no-store" });
      if (!res.ok) {
        setError(res.status === 404 ? "This status page doesn't exist." : "Unable to load this status page right now.");
        return;
      }
      setPage(await res.json());
      setError("");
    } catch {
      setError("Unable to load this status page right now.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
    const t = window.setInterval(load, 30000);
    return () => window.clearInterval(t);
  }, [params.slug]);

  if (loading) {
    return <main className="flex min-h-screen items-center justify-center bg-slate-950 text-slate-400">Loading…</main>;
  }
  if (error || !page) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-950 px-4 text-slate-100">
        <div className="max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-8 text-center">
          <div className="text-lg font-semibold">{error || "Status page not found"}</div>
        </div>
      </main>
    );
  }

  const overall = page.items.some(i => i.status === "down")
    ? "down"
    : page.items.some(i => i.status === "degraded")
    ? "degraded"
    : page.items.every(i => i.status === "up")
    ? "up"
    : "unknown";

  return (
    <main className="min-h-screen bg-slate-950 px-4 py-10 text-slate-100">
      <div className="mx-auto max-w-3xl">
        <div className="mb-8 text-center">
          <h1 className="text-3xl font-bold">{page.title}</h1>
          {page.description && <p className="mt-2 text-sm text-slate-400">{page.description}</p>}
        </div>

        <div className={`mb-8 flex items-center justify-center gap-3 rounded-2xl border p-5 ${overall === "up" ? "border-emerald-900 bg-emerald-950/30" : overall === "down" ? "border-red-900 bg-red-950/30" : "border-amber-900 bg-amber-950/30"}`}>
          <span className={`h-3 w-3 rounded-full ${STATUS_STYLE[overall].dot}`} />
          <span className={`text-lg font-semibold ${STATUS_STYLE[overall].text}`}>
            {overall === "up" ? "All systems operational" : overall === "down" ? "Experiencing an outage" : overall === "degraded" ? "Partial degradation" : "Status unknown"}
          </span>
        </div>

        <div className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-900">
          {page.items.length === 0 ? (
            <div className="p-8 text-center text-slate-500">No monitors configured on this page yet.</div>
          ) : (
            page.items.map((it, i) => (
              <div key={`${it.subjectType}-${it.subjectId}`} className={`flex items-center justify-between px-6 py-4 ${i !== 0 ? "border-t border-slate-800" : ""}`}>
                <span className="font-medium">{it.label}</span>
                <span className="flex items-center gap-4 text-sm">
                  {page.showCertificateExpiry && it.certExpiryDays != null && (
                    <span className={it.certExpiryDays <= 14 ? "text-amber-400" : "text-slate-500"}>
                      Cert expires in {it.certExpiryDays}d
                    </span>
                  )}
                  <span className={`flex items-center gap-2 font-semibold ${STATUS_STYLE[it.status].text}`}>
                    <span className={`h-2 w-2 rounded-full ${STATUS_STYLE[it.status].dot}`} />
                    {STATUS_STYLE[it.status].label}
                  </span>
                </span>
              </div>
            ))
          )}
        </div>

        <div className="mt-8 text-center text-xs text-slate-600">
          {page.footerText || "Powered by RoutingNMS"} · updated {new Date(page.generatedAt).toLocaleTimeString()}
        </div>
      </div>
    </main>
  );
}
