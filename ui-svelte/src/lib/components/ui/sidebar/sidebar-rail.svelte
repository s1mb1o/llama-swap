<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";
	import { SIDEBAR_WIDTH_MAX_PX, SIDEBAR_WIDTH_MIN_PX } from "./constants.js";
	import { useSidebar } from "./context.svelte.js";

	let {
		ref = $bindable(null),
		onResize,
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLButtonElement>, HTMLButtonElement> & {
		/** Called with the new width in px while the expanded sidebar is dragged. Without it the rail only toggles. */
		onResize?: (width: number) => void;
	} = $props();

	const sidebar = useSidebar();

	// A press that moves less than this many px stays a click, which toggles the sidebar.
	const DRAG_THRESHOLD_PX = 3;

	let drag: { startX: number; startWidth: number; dir: number; moved: boolean } | null = null;
	let suppressClick = false;

	function handlePointerDown(e: PointerEvent) {
		suppressClick = false;
		if (!onResize || !ref || e.button !== 0 || sidebar.state === "collapsed") return;
		const container = ref.closest<HTMLElement>('[data-slot="sidebar-container"]');
		if (!container) return;
		// Without this, Chrome starts a native drag of a menu link under the rail and cancels the pointer.
		e.preventDefault();
		ref.setPointerCapture(e.pointerId);
		drag = {
			startX: e.clientX,
			startWidth: container.getBoundingClientRect().width,
			dir: ref.closest('[data-side="right"]') ? -1 : 1,
			moved: false,
		};
	}

	function handlePointerMove(e: PointerEvent) {
		if (!drag) return;
		const dx = (e.clientX - drag.startX) * drag.dir;
		if (!drag.moved) {
			if (Math.abs(dx) < DRAG_THRESHOLD_PX) return;
			drag.moved = true;
			// index.css: col-resize cursor, no text selection, no width transition.
			document.body.classList.add("sidebar-resizing");
		}
		const width = Math.min(SIDEBAR_WIDTH_MAX_PX, Math.max(SIDEBAR_WIDTH_MIN_PX, drag.startWidth + dx));
		onResize?.(Math.round(width));
	}

	function handlePointerEnd() {
		if (!drag) return;
		// The click that follows a drag must not toggle the sidebar.
		suppressClick = drag.moved;
		drag = null;
		document.body.classList.remove("sidebar-resizing");
	}

	function handleClick() {
		if (suppressClick) {
			suppressClick = false;
			return;
		}
		sidebar.toggle();
	}
</script>

<button
	bind:this={ref}
	data-sidebar="rail"
	data-slot="sidebar-rail"
	aria-label="Toggle Sidebar"
	tabindex={-1}
	onpointerdown={handlePointerDown}
	onpointermove={handlePointerMove}
	onpointerup={handlePointerEnd}
	onpointercancel={handlePointerEnd}
	onclick={handleClick}
	title={onResize ? "Drag to resize, click to toggle" : "Toggle Sidebar"}
	class={cn(
		"hover:after:bg-sidebar-border absolute inset-y-0 z-20 hidden w-4 -translate-x-1/2 transition-all ease-linear group-data-[side=left]:-right-4 group-data-[side=right]:left-0 after:absolute after:inset-y-0 after:left-1/2 after:w-[2px] sm:flex",
		"in-data-[side=left]:cursor-w-resize in-data-[side=right]:cursor-e-resize",
		"[[data-side=left][data-state=collapsed]_&]:cursor-e-resize [[data-side=right][data-state=collapsed]_&]:cursor-w-resize",
		onResize && "touch-none [[data-state=expanded]_&]:cursor-col-resize",
		"hover:group-data-[collapsible=offcanvas]:bg-sidebar group-data-[collapsible=offcanvas]:translate-x-0 group-data-[collapsible=offcanvas]:after:left-full",
		"[[data-side=left][data-collapsible=offcanvas]_&]:-right-2",
		"[[data-side=right][data-collapsible=offcanvas]_&]:-left-2",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</button>
