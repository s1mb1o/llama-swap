<script lang="ts">
  import { link } from "svelte-spa-router";
  import { FerrisWheel, Boxes, Activity, ScrollText, Gauge, Sun, Moon, Monitor, ChevronRight, Settings } from "@lucide/svelte";
  import * as Sidebar from "$lib/components/ui/sidebar/index.js";
  import * as Collapsible from "$lib/components/ui/collapsible/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { cn } from "$lib/utils.js";
  import { toggleTheme, themeMode, appTitle } from "../stores/theme";
  import { currentRoute } from "../stores/route";
  import { playgroundActivity } from "../stores/playgroundActivity";
  import { performanceEnabled, models } from "../stores/api";
  import { showUnlistedModels } from "../stores/modelDisplay";
  import { modelsMenuOpen, openModelFolders } from "../stores/sidebar";
  import type { Model } from "../lib/types";
  import ConnectionStatus from "./ConnectionStatus.svelte";

  function handleTitleChange(newTitle: string): void {
    const sanitized = newTitle.replace(/\n/g, "").trim().substring(0, 64) || "llama-swap";
    appTitle.set(sanitized);
  }

  function handleKeyDown(e: KeyboardEvent): void {
    if (e.key === "Enter") {
      e.preventDefault();
      const target = e.currentTarget as HTMLElement;
      handleTitleChange(target.textContent || "(set title)");
      target.blur();
    }
  }

  function handleBlur(e: FocusEvent): void {
    const target = e.currentTarget as HTMLElement;
    handleTitleChange(target.textContent || "(set title)");
  }

  function isActive(path: string, current: string): boolean {
    return path === "/" ? current === "/" : current.startsWith(path);
  }

  let visibleModels = $derived(
    $showUnlistedModels ? $models : $models.filter((m) => !m.unlisted),
  );

  type DotColor = "grey" | "yellow" | "green";
  function statusDotColor(model: Model): DotColor {
    if (model.state === "ready") return "green";
    if (model.state === "starting" || model.state === "stopping") return "yellow";
    return "grey";
  }

  const dotClass: Record<DotColor, string> = {
    grey: "bg-muted-foreground/40",
    yellow: "bg-warning",
    green: "bg-success",
  };

  // Models grouped by metadata.folder. Untagged models go last, under "Other".
  const OTHER_FOLDER = "Other";
  let hasFolders = $derived(visibleModels.some((m) => m.folder));
  let modelFolders = $derived.by(() => {
    const byFolder = new Map<string, Model[]>();
    for (const model of visibleModels) {
      const name = model.folder || OTHER_FOLDER;
      byFolder.set(name, [...(byFolder.get(name) ?? []), model]);
    }
    return [...byFolder.entries()]
      .map(([name, models]) => ({ name, models }))
      .sort((a, b) => {
        if (a.name === OTHER_FOLDER) return 1;
        if (b.name === OTHER_FOLDER) return -1;
        return a.name.localeCompare(b.name, undefined, { numeric: true });
      });
  });

  function folderDotColor(folderModels: Model[]): DotColor {
    const colors = folderModels.map(statusDotColor);
    if (colors.includes("yellow")) return "yellow";
    if (colors.includes("green")) return "green";
    return "grey";
  }

  function setFolderOpen(name: string, open: boolean): void {
    openModelFolders.update((names) =>
      open ? [...names.filter((n) => n !== name), name] : names.filter((n) => n !== name),
    );
  }
</script>

{#snippet modelMenuItem(model: Model)}
  <Sidebar.MenuSubItem>
    <Sidebar.MenuSubButton
      isActive={$currentRoute === `/models/${encodeURIComponent(model.id)}`}
    >
      {#snippet child({ props })}
        <a href="/models/{encodeURIComponent(model.id)}" use:link {...props}>
          <span class={`size-2 shrink-0 rounded-full ${dotClass[statusDotColor(model)]}`}></span>
          <span class="flex-1 truncate">{model.id}</span>
        </a>
      {/snippet}
    </Sidebar.MenuSubButton>
  </Sidebar.MenuSubItem>
{/snippet}

<Sidebar.Root collapsible="icon">
  <Sidebar.Header>
    <div class="flex items-center gap-2 px-2 py-1.5">
      <div class="flex shrink-0 items-center justify-center">
        <ConnectionStatus />
      </div>
      <h1
        contenteditable="true"
        class="truncate pb-0 text-base font-semibold outline-none rounded-md px-1 hover:bg-sidebar-accent group-data-[collapsible=icon]:hidden"
        onblur={handleBlur}
        onkeydown={handleKeyDown}
      >
        {$appTitle}
      </h1>
    </div>
  </Sidebar.Header>

  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupContent>
        <Sidebar.Menu class="gap-1">
          <Sidebar.MenuItem>
            <Sidebar.MenuButton isActive={$currentRoute === "/" || isActive("/activity", $currentRoute)} tooltipContent="Activity">
              {#snippet child({ props })}
                <a href="/" use:link {...props}>
                  <Activity />
                  <span>Activity</span>
                </a>
              {/snippet}
            </Sidebar.MenuButton>
          </Sidebar.MenuItem>

          <Sidebar.MenuItem>
            <Sidebar.MenuButton isActive={isActive("/playground", $currentRoute)} tooltipContent="Playground">
              {#snippet child({ props })}
                <a href="/playground" use:link {...props}>
                  <FerrisWheel />
                  <span class={$playgroundActivity ? "activity-link" : ""}>Playground</span>
                </a>
              {/snippet}
            </Sidebar.MenuButton>
          </Sidebar.MenuItem>

          <Sidebar.MenuItem>
            <Collapsible.Root
              open={$modelsMenuOpen}
              onOpenChange={(v) => modelsMenuOpen.set(v)}
              class="gap-0"
            >
              <Sidebar.MenuButton
                isActive={$currentRoute.startsWith("/models")}
                tooltipContent="Models"
              >
                {#snippet child({ props })}
                  <a href="/models" use:link {...props}>
                    <Boxes />
                    <span>Models</span>
                    <span
                      class="ml-auto transition-transform duration-200 {$modelsMenuOpen ? 'rotate-90' : ''}"
                      role="button"
                      tabindex="0"
                      aria-label="Toggle models section"
                      onclick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        modelsMenuOpen.update((v) => !v);
                      }}
                      onkeydown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          e.stopPropagation();
                          modelsMenuOpen.update((v) => !v);
                        }
                      }}
                    >
                      <ChevronRight />
                    </span>
                  </a>
                {/snippet}
              </Sidebar.MenuButton>
              <Collapsible.Content>
                <Sidebar.MenuSub>
                  {#if hasFolders}
                    {#each modelFolders as folder (folder.name)}
                      {@const isOpen = $openModelFolders.includes(folder.name)}
                      <Sidebar.MenuSubItem>
                        <Collapsible.Root open={isOpen} onOpenChange={(v) => setFolderOpen(folder.name, v)}>
                          <Sidebar.MenuSubButton>
                            {#snippet child({ props })}
                              <button
                                type="button"
                                {...props}
                                class={cn(props.class as string, "w-full")}
                                aria-expanded={isOpen}
                                onclick={() => setFolderOpen(folder.name, !isOpen)}
                              >
                                <span class={`size-2 shrink-0 rounded-full ${dotClass[folderDotColor(folder.models)]}`}></span>
                                <span class="flex-1 truncate text-left">{folder.name}</span>
                                <span class="text-muted-foreground text-xs tabular-nums">{folder.models.length}</span>
                                <ChevronRight class="transition-transform duration-200 {isOpen ? 'rotate-90' : ''}" />
                              </button>
                            {/snippet}
                          </Sidebar.MenuSubButton>
                          <Collapsible.Content>
                            <Sidebar.MenuSub class="mr-0 pr-0">
                              {#each folder.models as model (model.id)}
                                {@render modelMenuItem(model)}
                              {/each}
                            </Sidebar.MenuSub>
                          </Collapsible.Content>
                        </Collapsible.Root>
                      </Sidebar.MenuSubItem>
                    {/each}
                  {:else}
                    {#each visibleModels as model (model.id)}
                      {@render modelMenuItem(model)}
                    {/each}
                  {/if}
                </Sidebar.MenuSub>
              </Collapsible.Content>
            </Collapsible.Root>
          </Sidebar.MenuItem>

          <Sidebar.MenuItem>
            <Sidebar.MenuButton isActive={isActive("/logs", $currentRoute)} tooltipContent="Logs">
              {#snippet child({ props })}
                <a href="/logs" use:link {...props}>
                  <ScrollText />
                  <span>Logs</span>
                </a>
              {/snippet}
            </Sidebar.MenuButton>
          </Sidebar.MenuItem>

          {#if $performanceEnabled}
            <Sidebar.MenuItem>
              <Sidebar.MenuButton isActive={isActive("/performance", $currentRoute)} tooltipContent="Performance">
                {#snippet child({ props })}
                  <a href="/performance" use:link {...props}>
                    <Gauge />
                    <span>Performance</span>
                  </a>
                {/snippet}
              </Sidebar.MenuButton>
            </Sidebar.MenuItem>
          {/if}
        </Sidebar.Menu>
      </Sidebar.GroupContent>
    </Sidebar.Group>
  </Sidebar.Content>

  <Sidebar.Footer>
    <div
      class="flex items-center justify-between gap-2 px-1 group-data-[collapsible=icon]:flex-col-reverse"
    >
      <Sidebar.MenuButton
        isActive={isActive("/settings", $currentRoute)}
        tooltipContent="Settings"
      >
        {#snippet child({ props })}
          <a href="/settings" use:link {...props}>
            <Settings />
            <span>Settings</span>
          </a>
        {/snippet}
      </Sidebar.MenuButton>
      <Button
        variant="ghost"
        size="icon"
        onclick={toggleTheme}
        title="Toggle theme (current: {$themeMode})"
      >
        {#if $themeMode === "system"}
          <Monitor />
        {:else if $themeMode === "light"}
          <Sun />
        {:else}
          <Moon />
        {/if}
        <span class="sr-only">Toggle theme</span>
      </Button>
    </div>
  </Sidebar.Footer>
  <Sidebar.Rail />
</Sidebar.Root>
