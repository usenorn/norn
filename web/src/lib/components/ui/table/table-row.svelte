<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLTableRowElement>> = $props();
</script>

<tr bind:this={ref} data-slot="table-row" class={cn("transition-colors duration-70 ease-out data-[state=selected]:bg-surface-selected data-[state=selected]:rule-lead", className)} {...restProps}>
	{@render children?.()}
</tr>
