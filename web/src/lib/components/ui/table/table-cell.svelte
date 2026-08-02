<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLTdAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLTdAttributes> = $props();
</script>

<td bind:this={ref} data-slot="table-cell" class={cn("h-7.5 border-b border-line-subtle px-2.5 align-middle tracking-snug whitespace-nowrap data-[align=right]:text-right", className)} {...restProps}>
	{@render children?.()}
</td>
