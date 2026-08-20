<script lang="ts">
	import { MorphIcon, type IconNode } from "morphicons/svelte";
	import { cn, type IconComponent } from "$lib/utils.js";

	let {
		href,
		label,
		icon,
		iconClass,
		glyph,
		glyphEngaged,
		dot,
		count,
		active = false,
		indent = false,
		class: className,
	}: {
		href: string;
		label: string;
		icon?: IconComponent;
		iconClass?: string;
		glyph?: IconNode;
		glyphEngaged?: IconNode;
		dot?: string;
		count?: number;
		active?: boolean;
		indent?: boolean;
		class?: string;
	} = $props();

	let hovered = $state(false);
	const engaged = $derived(hovered && glyphEngaged ? glyphEngaged : glyph);
</script>

<a
	{href}
	data-active={active}
	aria-current={active ? "page" : undefined}
	onpointerenter={() => (hovered = true)}
	onpointerleave={() => (hovered = false)}
	class={cn(
		"flex h-6.75 w-full items-center gap-2 rounded-xs px-2 text-md font-medium tracking-snug text-ink-600 motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring data-[active=true]:rule-inset data-[active=true]:bg-accent data-[active=true]:text-ink-900",
		indent && "pl-6",
		className
	)}
>
	{#if dot}
		<span
			class="size-2 shrink-0 rounded-xs"
			style="background: {dot}"
			aria-hidden="true"
		></span>
	{:else if glyph}
		<MorphIcon
			icon={engaged}
			spring="snappy"
			reducedMotion="user"
			class={cn("size-icon-row shrink-0", iconClass)}
			aria-hidden="true"
		/>
	{:else if icon}
		{@const Glyph = icon}
		<Glyph class={cn("size-icon-row shrink-0", iconClass)} aria-hidden="true" />
	{/if}
	<span class="min-w-0 flex-1 truncate">{label}</span>
	{#if count !== undefined}
		<span class="font-mono text-xs text-muted-foreground tabular-nums">{count}</span>
	{/if}
</a>
