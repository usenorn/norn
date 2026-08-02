<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLTableAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLTableAttributes> = $props();
</script>

<div
	data-slot="table-container"
	class="relative w-full overflow-x-auto rounded-lg border border-line-default bg-card"
>
	<table
		bind:this={ref}
		data-slot="table"
		class={cn("w-full caption-bottom border-collapse text-md text-foreground", className)}
		{...restProps}
	>
		{@render children?.()}
	</table>
</div>
