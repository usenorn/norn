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
		stretched = false,
		size = "row",
		onclick,
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
		stretched?: boolean;
		size?: "row" | "touch";
		onclick?: (event: MouseEvent) => void;
		class?: string;
	} = $props();

	let hovered = $state(false);
	const engaged = $derived(hovered && glyphEngaged ? glyphEngaged : glyph);
</script>

<a
	{href}
	data-slot={stretched ? undefined : "sidebar-row"}
	data-active={active}
	aria-current={active ? "page" : undefined}
	onpointerenter={() => (hovered = true)}
	onpointerleave={() => (hovered = false)}
	{onclick}
	class={cn(
		"flex items-center gap-2 rounded-xs px-2 text-md font-medium tracking-snug text-ink-600 motion-control hover:text-ink-900 data-[active=true]:text-ink-900",
		stretched
			? "after:absolute after:inset-0 after:rounded-xs after:content-[''] focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:outline-offset-[-2px] focus-visible:after:outline-ring"
			: "w-full hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring data-[active=true]:rule-inset data-[active=true]:bg-accent",
		size === "touch" ? "h-11 gap-3 rounded-sm" : "h-6.75",
		indent && (size === "touch" ? "pl-8" : "pl-7"),
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
