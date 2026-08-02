<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLThAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLThAttributes> = $props();
</script>

<th bind:this={ref} data-slot="table-head" class={cn("h-6.75 border-b border-line-default bg-card px-2.5 text-left align-middle font-mono text-2xs font-medium tracking-caps whitespace-nowrap text-ink-600 uppercase data-[align=right]:text-right", className)} {...restProps}>
	{@render children?.()}
</th>
