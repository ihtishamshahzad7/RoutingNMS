"use client";

import { useEffect, useState } from "react";
import { apiFetch, ApiError } from "../lib/api";

type Point = { timestamp: string; value: number };
type Series = { metric: string; points: Point[] };

/**
 * Minimal dependency-free SVG line chart for one metric's recent history.
 * Deliberately avoids pulling in a charting library (recharts, chart.js,
 * ...) to keep the frontend's dependency footprint and production build
 * unchanged -- this project has gotten by on hand-rolled SVG/canvas UI
 * throughout, and a sparkline doesn't need more than that.
 */
export function MetricChart({
  subjectType,
  subjectId,
  metric,
  label,
  unit = "",
  since = "24h",
  formatValue,
}: {
  subjectType: string;
  subjectId: string;
  metric: string;
  label: string;
  unit?: string;
  since?: string;
  formatValue?: (v: number) => string;
}) {
  const [points, setPoints] = useState<Point[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    apiFetch<Series[]>(`/metrics?subjectType=${subjectType}&subjectId=${encodeURIComponent(subjectId)}&metric=${metric}&since=${since}`)
      .then(series => { if (active) setPoints(series[0]?.points ?? []); })
      .catch(err => { if (active) setError(err instanceof ApiError ? err.message : "Unable to load metric history."); });
    return () => { active = false; };
  }, [subjectType, subjectId, metric, since]);

  const fmt = formatValue ?? ((v: number) => v.toFixed(1));

  if (error) return <div className="text-xs text-red-400">{error}</div>;
  if (points === null) return <div className="text-xs text-slate-500">Loading…</div>;
  if (points.length === 0) return <div className="text-xs text-slate-500">No history yet for {label.toLowerCase()}.</div>;

  const width = 320, height = 72, pad = 4;
  const values = points.map(p => p.value);
  const min = Math.min(...values), max = Math.max(...values);
  const range = max - min || 1;
  const stepX = points.length > 1 ? (width - pad * 2) / (points.length - 1) : 0;
  const coords = points.map((p, i) => {
    const x = pad + i * stepX;
    const y = height - pad - ((p.value - min) / range) * (height - pad * 2);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const latest = values[values.length - 1];

  return (
    <div>
      <div className="mb-1 flex items-baseline justify-between">
        <span className="text-xs text-slate-400">{label}</span>
        <span className="font-mono text-sm text-slate-200">{fmt(latest)}{unit}</span>
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full text-cyan-400" preserveAspectRatio="none">
        <polyline points={coords.join(" ")} fill="none" stroke="currentColor" strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
      </svg>
      <div className="flex justify-between text-[10px] text-slate-600">
        <span>{fmt(min)}{unit}</span>
        <span>{fmt(max)}{unit}</span>
      </div>
    </div>
  );
}
