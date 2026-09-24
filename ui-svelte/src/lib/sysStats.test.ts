import { describe, it, expect } from "vitest";
import { cpuAvgPct, memUsedPct, swapUsedPct, sparklinePath } from "./sysStats";
import type { SysStat } from "./types";

function stat(overrides: Partial<SysStat> = {}): SysStat {
  return {
    timestamp: "2026-09-24T20:00:00Z",
    cpu_util_per_core: [],
    mem_total_mb: 0,
    mem_used_mb: 0,
    mem_free_mb: 0,
    swap_total_mb: 0,
    swap_used_mb: 0,
    load_avg_1: 0,
    load_avg_5: 0,
    load_avg_15: 0,
    net_io: [],
    ...overrides,
  };
}

describe("cpuAvgPct", () => {
  it("averages the per-core values", () => {
    expect(cpuAvgPct(stat({ cpu_util_per_core: [10, 20, 30, 40] }))).toBe(25);
  });

  it("returns 0 when no cores are reported", () => {
    expect(cpuAvgPct(stat())).toBe(0);
  });
});

describe("memUsedPct / swapUsedPct", () => {
  it("computes used percent of total", () => {
    expect(memUsedPct(stat({ mem_total_mb: 200, mem_used_mb: 50 }))).toBe(25);
    expect(swapUsedPct(stat({ swap_total_mb: 400, swap_used_mb: 100 }))).toBe(25);
  });

  it("handles a zero total", () => {
    expect(memUsedPct(stat())).toBe(0);
    expect(swapUsedPct(stat())).toBeNull();
  });
});

describe("sparklinePath", () => {
  it("maps the time window to x and the value range to inverted y", () => {
    const path = sparklinePath(
      [
        { t: 0, v: 0 },
        { t: 50, v: 50 },
        { t: 100, v: 100 },
      ],
      100,
      100,
    );
    expect(path).toBe("M0.00,100.00 L50.00,50.00 L100.00,0.00");
  });

  it("drops points outside the window and clamps values", () => {
    const path = sparklinePath(
      [
        { t: 10, v: 50 },
        { t: 60, v: -5 },
        { t: 110, v: 150 },
      ],
      110,
      50,
    );
    expect(path).toBe("M0.00,100.00 L100.00,0.00");
  });

  it("returns an empty path for no points", () => {
    expect(sparklinePath([], 100, 100)).toBe("");
  });
});
