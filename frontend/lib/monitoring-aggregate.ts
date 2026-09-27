// Shared bucketing for the Connectivity Monitoring detail page's charts
// (6-page rebuild, item 2.3): the 1h view stays raw/near-raw (short enough
// to read as-is), but 24h buckets into one point per hour and 7d buckets
// into one point per day -- average latency and uptime % per bucket --
// instead of a scatter of every raw probe. Used identically by the ICMP
// and Port/Service charts so the two aggregate the same way.

export type RawPoint = { t: number; latencyMs: number | null; reachable: boolean };
export type AggregatedPoint = { t: number; avgLatencyMs: number | null; uptimePct: number; reachable: boolean };

export type Range = "1h" | "24h" | "7d";

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;

/** Bucket size for a given range: null means "no bucketing, return raw." */
function bucketSizeFor(range: Range): number | null {
  switch (range) {
    case "24h":
      return HOUR_MS;
    case "7d":
      return DAY_MS;
    default:
      return null;
  }
}

/** Aggregates raw probe points into per-bucket average latency + uptime %,
 * bucket-aligned to the hour (24h) or the calendar day in the browser's
 * local time zone (7d) so bucket boundaries land on readable times rather
 * than an arbitrary offset from "now". The 1h view returns the raw points
 * unbucketed (one "bucket" per point) since it's already short enough to
 * read directly. */
export function aggregate(points: RawPoint[], range: Range): AggregatedPoint[] {
  const bucketMs = bucketSizeFor(range);
  if (bucketMs == null) {
    return points.map((p) => ({ t: p.t, avgLatencyMs: p.latencyMs, uptimePct: p.reachable ? 100 : 0, reachable: p.reachable }));
  }

  const buckets = new Map<number, { latencies: number[]; reachableCount: number; total: number }>();
  for (const p of points) {
    const bucketStart = Math.floor(p.t / bucketMs) * bucketMs;
    let b = buckets.get(bucketStart);
    if (!b) {
      b = { latencies: [], reachableCount: 0, total: 0 };
      buckets.set(bucketStart, b);
    }
    b.total += 1;
    if (p.reachable) b.reachableCount += 1;
    if (p.latencyMs != null) b.latencies.push(p.latencyMs);
  }

  return Array.from(buckets.entries())
    .sort(([a], [b]) => a - b)
    .map(([t, b]) => {
      const uptimePct = b.total ? (b.reachableCount / b.total) * 100 : 0;
      const avgLatencyMs = b.latencies.length ? b.latencies.reduce((s, v) => s + v, 0) / b.latencies.length : null;
      return { t, avgLatencyMs, uptimePct, reachable: uptimePct >= 50 };
    });
}
