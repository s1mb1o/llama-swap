<script lang="ts">
  import { onMount } from "svelte";
  import { link } from "svelte-spa-router";
  import { fetchPerformance, performanceEnabled } from "../stores/api";
  import { cpuAvgPct, memUsedPct, swapUsedPct, gpuUtilSeries, sparklinePath } from "../lib/sysStats";
  import type { GpuStat, SysStat } from "../lib/types";

  // Sparkline history shown in the sidebar, and how often it is refreshed.
  const WINDOW_MS = 10 * 60 * 1000;
  const POLL_MS = 5000;

  let stats = $state<SysStat[]>([]);
  let gpu = $state<GpuStat[]>([]);
  let enabled = $state(false);
  let timer: ReturnType<typeof setInterval> | null = null;
  let inFlight = false;

  const ts = (x: { timestamp: string }) => Date.parse(x.timestamp);
  const lastT = (xs: { timestamp: string }[]) => (xs.length > 0 ? ts(xs[xs.length - 1]) : 0);

  async function poll(): Promise<void> {
    if (inFlight) return;
    inFlight = true;
    try {
      // System and GPU samples come from separate tickers, so each list keeps
      // its own cursor. Ask from the older one and drop what is already held.
      const sysT = lastT(stats);
      const gpuT = lastT(gpu);
      const since = Math.min(sysT || Infinity, gpuT || Infinity);
      const after = new Date(Number.isFinite(since) ? since : Date.now() - WINDOW_MS).toISOString();
      const resp = await fetchPerformance(after);
      const newSys = (resp?.sys_stats ?? []).filter((s) => ts(s) > sysT);
      const newGpu = (resp?.gpu_stats ?? []).filter((g) => ts(g) > gpuT);
      if (newSys.length === 0 && newGpu.length === 0) return;
      const sysAll = [...stats, ...newSys];
      const gpuAll = [...gpu, ...newGpu];
      const cutoff = Math.max(lastT(sysAll), lastT(gpuAll)) - WINDOW_MS;
      stats = sysAll.filter((s) => ts(s) >= cutoff);
      gpu = gpuAll.filter((g) => ts(g) >= cutoff);
    } finally {
      inFlight = false;
    }
  }

  function start(): void {
    stop();
    poll();
    timer = setInterval(poll, POLL_MS);
  }

  function stop(): void {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
  }

  function sync(): void {
    if (enabled && !document.hidden) start();
    else stop();
  }

  onMount(() => {
    const unsubscribe = performanceEnabled.subscribe((v) => {
      enabled = v;
      sync();
    });
    document.addEventListener("visibilitychange", sync);
    return () => {
      unsubscribe();
      stop();
      document.removeEventListener("visibilitychange", sync);
    };
  });

  let latest = $derived(stats.at(-1));
  let endT = $derived(Math.max(lastT(stats), lastT(gpu)));

  function path(value: (s: SysStat) => number): string {
    return sparklinePath(
      stats.map((s) => ({ t: ts(s), v: value(s) })),
      endT,
      WINDOW_MS,
    );
  }

  let gpuSeries = $derived(gpuUtilSeries(gpu));
  let gpuPaths = $derived(gpuSeries.map((points) => sparklinePath(points, endT, WINDOW_MS)));
  // Mean of each GPU's latest utilization.
  let gpuUtil = $derived(
    gpuSeries.length > 0 ? gpuSeries.reduce((sum, points) => sum + points[points.length - 1].v, 0) / gpuSeries.length : 0,
  );
  let cpuPath = $derived(path(cpuAvgPct));
  let memPath = $derived(path(memUsedPct));
  let swapPath = $derived(latest && swapUsedPct(latest) !== null ? path((s) => swapUsedPct(s) ?? 0) : "");

  const gib = (mb: number) => (mb / 1024).toFixed(1);
</script>

{#snippet sparkline(lines: { d: string; class: string }[])}
  <svg
    class="mt-1 h-8 w-full rounded-sm bg-sidebar-accent/50"
    viewBox="0 0 100 100"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    {#each lines as line}
      <path
        d={line.d}
        class={line.class}
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linejoin="round"
        vector-effect="non-scaling-stroke"
      />
    {/each}
  </svg>
{/snippet}

{#if enabled && latest}
  {@const swapPct = swapUsedPct(latest)}
  <a
    href="/performance"
    use:link
    title="Open Performance (last 10 min)"
    class="block rounded-md px-2 py-1.5 text-xs hover:bg-sidebar-accent group-data-[collapsible=icon]:hidden"
  >
    {#if gpuSeries.length > 0}
      <div class="mb-2">
        <div class="flex items-baseline justify-between gap-2">
          <span class="font-medium">GPU</span>
          <span class="tabular-nums text-sidebar-foreground">{gpuUtil.toFixed(0)}%</span>
        </div>
        {@render sparkline(gpuPaths.map((d) => ({ d, class: "text-primary" })))}
      </div>
    {/if}

    <div class="flex items-baseline justify-between gap-2">
      <span class="font-medium">CPU</span>
      <span
        class="tabular-nums text-muted-foreground"
        title="Load average 1 / 5 / 15 min: {latest.load_avg_1.toFixed(2)} / {latest.load_avg_5.toFixed(2)} / {latest.load_avg_15.toFixed(2)}"
      >
        <span class="text-sidebar-foreground">{cpuAvgPct(latest).toFixed(1)}%</span> · load {latest.load_avg_1.toFixed(2)}
      </span>
    </div>
    {@render sparkline([{ d: cpuPath, class: "text-primary" }])}

    <div class="mt-2 flex items-baseline justify-between gap-2">
      <span class="font-medium">Memory</span>
      <span class="tabular-nums text-muted-foreground">
        <span class="text-sidebar-foreground">{gib(latest.mem_used_mb)}</span> / {gib(latest.mem_total_mb)} GiB
      </span>
    </div>
    {@render sparkline([
      { d: swapPath, class: "text-warning" },
      { d: memPath, class: "text-primary" },
    ])}
    <div class="mt-1 flex gap-3 tabular-nums text-muted-foreground">
      <span class="flex items-center gap-1">
        <span class="size-2 rounded-full bg-primary"></span>RAM {memUsedPct(latest).toFixed(0)}%
      </span>
      {#if swapPct !== null}
        <span
          class="flex items-center gap-1"
          title="Swap {gib(latest.swap_used_mb)} / {gib(latest.swap_total_mb)} GiB"
        >
          <span class="size-2 rounded-full bg-warning"></span>Swap {swapPct.toFixed(0)}%
        </span>
      {/if}
    </div>
  </a>
{/if}
