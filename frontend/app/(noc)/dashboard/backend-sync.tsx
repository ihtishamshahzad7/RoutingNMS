"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

type Check = { name: string; path: string; ok: boolean; detail: string; href?: string };

export default function BackendSync() {
  const [checks, setChecks] = useState<Check[]>([]);

  useEffect(() => {
    let active = true;
    const run = async () => {
      const endpoints = [
        ["API health", "/api/v1/health"],
        ["API readiness", "/api/v1/ready"],
        ["Devices API", "/api/v1/devices?organizationId=tenant-1"],
        ["OLT API", "/api/v1/olts"],
        ["Topology API", "/api/topology"],
        ["Incidents API", "/api/incidents"],
      ] as const;
      const results = await Promise.all(endpoints.map(async ([name, path]) => {
        try {
          const response = await fetch(path, { cache: "no-store" });
          return { name, path, ok: response.ok || response.status === 401, detail: response.status === 401 ? "Protected endpoint reachable" : `${response.status} ${response.statusText}` };
        } catch {
          return { name, path, ok: false, detail: "Connection failed" };
        }
      }));
      if (active) setChecks(results);
    };
    run();
    const timer = window.setInterval(run, 15000);
    return () => { active = false; window.clearInterval(timer); };
  }, []);

  return (
    <section className="mt-6 rounded-xl border border-slate-800 bg-slate-900 p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-semibold">Backend ↔ Frontend Sync</h2>
          <p className="mt-1 text-xs text-slate-500">Live checks confirm that the frontend can reach the backend APIs through Nginx.</p>
        </div>
        <Link href="/devices" className="text-xs text-cyan-400">Open device management →</Link>
      </div>
      <div className="mt-4 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
        {checks.map(check => (
          <div key={check.path} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-slate-200">{check.name}</span>
              <span className={`rounded-full px-2 py-1 text-[11px] ${check.ok ? "bg-emerald-950 text-emerald-300" : "bg-red-950 text-red-300"}`}>{check.ok ? "OK" : "FAILED"}</span>
            </div>
            <div className="mt-1 text-[11px] text-slate-500">{check.path} · {check.detail}</div>
          </div>
        ))}
      </div>
    </section>
  );
}
