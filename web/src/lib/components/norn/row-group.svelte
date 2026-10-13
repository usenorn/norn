<script lang="ts">
	import type { HTMLAttributes } from "svelte/elements";
	import { cn } from "$lib/utils.js";

	let {
		as = "div",
		class: className,
		children,
		...restProps
	}: HTMLAttributes<HTMLElement> & { as?: "div" | "ul" } = $props();
</script>

<svelte:element
	this={as}
	class={cn("flex flex-col divide-y divide-line-default rounded-md border border-line-default", className)}
	{...restProps}
>
	{@render children?.()}
</svelte:element>
