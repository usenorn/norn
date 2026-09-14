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
		ontoggle: () => void;
		action?: Snippet;
		children: Snippet;
		class?: string;
	} = $props();
</script>

<div class={cn("relative flex items-center", className)}>
	<SidebarItem
		{href}
		{label}
		{icon}
		{glyph}
		{glyphEngaged}
		{active}
		class={cn("min-w-0 flex-1 pl-7", action && "pr-8")}
	/>
	<button
		type="button"
		onclick={ontoggle}
		aria-expanded={expanded}
		aria-controls={id}
		aria-label="{expanded ? 'Collapse' : 'Expand'} {label}"
		class="absolute top-1/2 left-1.5 flex size-5 -translate-y-1/2 items-center justify-center rounded-xs text-ink-600 motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
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
		<span class="absolute top-1/2 right-1 -translate-y-1/2">
			{@render action()}
		</span>
	{/if}
</div>
{#if expanded}
	<div {id}>
		{@render children()}
	</div>
{/if}
