<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		inset,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> & {
		inset?: boolean;
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="dropdown-menu-label"
	data-inset={inset}
	class={cn(
		"flex items-center gap-1.5 px-2 pt-1.5 pb-1 font-mono text-2xs font-medium tracking-caps text-ink-600 uppercase data-inset:pl-8 data-[inset]:pl-8",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</div>
