<script lang="ts">
	import type { Snippet } from "svelte";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import SidebarItem from "./sidebar-item.svelte";
	import { cn, type IconComponent } from "$lib/utils.js";

	let {
		id,
		href,
		label,
		icon,
		active = false,
		expanded = false,
		ontoggle,
		children,
		class: className,
	}: {
		id: string;
		href: string;
		label: string;
		icon?: IconComponent;
		active?: boolean;
		expanded?: boolean;
		ontoggle: () => void;
		children: Snippet;
		class?: string;
	} = $props();
</script>

<div class={cn("flex items-center gap-0.5", className)}>
	<button
		type="button"
		onclick={ontoggle}
		aria-expanded={expanded}
		aria-controls={id}
		aria-label="{expanded ? 'Collapse' : 'Expand'} {label}"
		class="flex size-5 shrink-0 items-center justify-center rounded-xs text-ink-600 motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
	>
		<ChevronRight
			class="size-3 motion-control {expanded ? 'rotate-90' : ''}"
			aria-hidden="true"
		/>
	</button>
	<SidebarItem {href} {label} {icon} {active} class="min-w-0 flex-1" />
</div>
{#if expanded}
	<div {id} class="pl-5">
		{@render children()}
	</div>
{/if}
