<script lang="ts">
	import { cn, type IconComponent } from "$lib/utils.js";

	let {
		href,
		label,
		icon,
		iconClass,
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
		dot?: string;
		count?: number;
		active?: boolean;
		indent?: boolean;
		class?: string;
	} = $props();
</script>

<a
	{href}
	data-active={active}
	aria-current={active ? "page" : undefined}
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
	{:else if icon}
		{@const Glyph = icon}
		<Glyph class={cn("size-icon-row shrink-0", iconClass)} aria-hidden="true" />
	{/if}
	<span class="min-w-0 flex-1 truncate">{label}</span>
	{#if count !== undefined}
		<span class="font-mono text-xs text-muted-foreground tabular-nums">{count}</span>
	{/if}
</a>
