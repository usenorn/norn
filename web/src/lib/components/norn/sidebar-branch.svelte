<script lang="ts">
	import type { Snippet } from "svelte";
	import { MorphIcon, type IconNode } from "morphicons/svelte";
	import { ChevronDown, ChevronRight } from "lucide";
	import SidebarItem from "./sidebar-item.svelte";
	import { cn, type IconComponent } from "$lib/utils.js";

	let {
		id,
		href,
		label,
		icon,
		glyph,
		glyphEngaged,
		active = false,
		expanded = false,
		size = "row",
		ontoggle,
		action,
		children,
		class: className,
	}: {
		id: string;
		href: string;
		label: string;
		icon?: IconComponent;
		glyph?: IconNode;
		glyphEngaged?: IconNode;
		active?: boolean;
		expanded?: boolean;
		size?: "row" | "touch";
		ontoggle: () => void;
		action?: Snippet;
		children: Snippet;
		class?: string;
	} = $props();
</script>

<div
	data-slot="sidebar-row"
	data-active={active}
	class={cn(
		"relative isolate flex items-center gap-0.5 pr-1 motion-control hover:bg-accent data-[active=true]:rule-inset data-[active=true]:bg-accent",
		size === "touch" ? "h-11 rounded-sm" : "h-6.75 rounded-xs",
		className
	)}
>
	<SidebarItem {href} {label} {icon} {glyph} {glyphEngaged} {active} {size} stretched class="min-w-0" />
	<button
		type="button"
		onclick={ontoggle}
		aria-expanded={expanded}
		aria-controls={id}
		aria-label="{expanded ? 'Collapse' : 'Expand'} {label}"
		class="relative z-1 flex shrink-0 items-center justify-center rounded-xs text-ink-600 {size === 'touch' ? 'size-9' : 'size-5'} motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
	>
		<MorphIcon
			icon={expanded ? ChevronDown : ChevronRight}
			spring="snappy"
			reducedMotion="user"
			class="size-3"
			aria-hidden="true"
		/>
	</button>
	{#if action}
		<span class="relative z-1 ml-auto shrink-0">
			{@render action()}
		</span>
	{/if}
</div>
{#if expanded}
	<div {id}>
		{@render children()}
	</div>
{/if}
