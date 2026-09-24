// Helpers for the sidebar GPU / CPU / memory sparklines.
import type { GpuStat, SysStat } from "./types";

/** Mean utilization across all cores, in percent. */
export function cpuAvgPct(s: SysStat): number {
  const cores = s.cpu_util_per_core ?? [];
  if (cores.length === 0) return 0;
  return cores.reduce((sum, v) => sum + v, 0) / cores.length;
}

/** RAM used, in percent of total. */
export function memUsedPct(s: SysStat): number {
  return s.mem_total_mb > 0 ? (s.mem_used_mb / s.mem_total_mb) * 100 : 0;
}

/** Swap used, in percent of total; null when the host has no swap. */
export function swapUsedPct(s: SysStat): number | null {
  return s.swap_total_mb > 0 ? (s.swap_used_mb / s.swap_total_mb) * 100 : null;
}

export interface SparkPoint {
  t: number; // epoch ms
  v: number;
}

/**
 * GPU utilization points, one series per GPU id, in first-seen order.
 * Each GPU in a snapshot has its own timestamp, so series are kept apart
 * instead of being averaged per timestamp.
 */
export function gpuUtilSeries(gpu: GpuStat[]): SparkPoint[][] {
  const byId = new Map<number, SparkPoint[]>();
  for (const g of gpu) {
    const points = byId.get(g.id) ?? [];
    points.push({ t: Date.parse(g.timestamp), v: g.gpu_util_pct });
    byId.set(g.id, points);
  }
  return [...byId.values()];
}

/**
 * Build an SVG path for a "0 0 100 100" viewBox. x maps the time window
 * [endT - windowMs, endT] to [0, 100]; y maps [0, max] to [100, 0], clamped.
 * Points outside the window are dropped.
 */
export function sparklinePath(points: SparkPoint[], endT: number, windowMs: number, max = 100): string {
  const startT = endT - windowMs;
  return points
    .filter((p) => p.t >= startT && p.t <= endT)
    .map((p, i) => {
      const x = ((p.t - startT) / windowMs) * 100;
      const y = 100 - Math.min(Math.max(p.v / max, 0), 1) * 100;
      return `${i === 0 ? "M" : "L"}${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");
}
