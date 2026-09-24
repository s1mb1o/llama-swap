<script lang="ts">
  import { onMount } from "svelte";
  import { link } from "svelte-spa-router";
  import { fetchPerformance, performanceEnabled } from "../stores/api";
  import { cpuAvgPct, memUsedPct, swapUsedPct, sparklinePath } from "../lib/sysStats";
  import type { SysStat } from "../lib/types";

  // Sparkline history shown in the sidebar, and how often it is refreshed.
  const WINDOW_MS = 10 * 60 * 1000;
  const POLL_MS = 5000;

  let stats = $state<SysStat[]>([]);
  let enabled = $state(false);
  let timer: ReturnType<typeof setInterval> | null = null;
  let inFlight = false;

  async function poll(): Promise<void> {
    if (inFlight) return;
    inFlight = true;
    try {
      const last = stats.at(-1)?.timestamp;
      const after = last ?? new Date(Date.now() - WINDOW_MS).toISOString();
      const fresh = (await fetchPerformance(after))?.sys_stats ?? [];
      if (fresh.length === 0) return;
      const merged = [...stats, ...fresh];
      const endT = Date.parse(merged[merged.length - 1].timestamp);
      stats = merged.filter((s) => Date.parse(s.timestamp) >= endT - WINDOW_MS);
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
  let endT = $derived(latest ? Date.parse(latest.timestamp) : 0);

  function path(value: (s: SysStat) => number): string {
    return sparklinePath(
      stats.map((s) => ({ t: Date.parse(s.timestamp), v: value(s) })),
      endT,
      WINDOW_MS,
    );
  }

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
