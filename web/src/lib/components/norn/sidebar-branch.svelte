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

<div
	data-slot="sidebar-row"
	data-active={active}
	class={cn(
		"relative isolate flex h-6.75 items-center gap-0.5 rounded-xs pr-1 motion-control hover:bg-accent data-[active=true]:rule-inset data-[active=true]:bg-accent",
		className
	)}
>
	<SidebarItem {href} {label} {icon} {glyph} {glyphEngaged} {active} stretched class="min-w-0" />
	<button
		type="button"
		onclick={ontoggle}
		aria-expanded={expanded}
		aria-controls={id}
		aria-label="{expanded ? 'Collapse' : 'Expand'} {label}"
		class="relative z-1 flex size-5 shrink-0 items-center justify-center rounded-xs text-ink-600 motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
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
