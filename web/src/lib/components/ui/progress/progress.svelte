<script lang="ts">
	import { Progress as ProgressPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		max = 100,
		value,
		indeterminate = false,
		...restProps
	}: WithoutChildrenOrChild<ProgressPrimitive.RootProps> & { indeterminate?: boolean } = $props();
</script>

<ProgressPrimitive.Root
	bind:ref
	data-slot="progress"
	class={cn(
		"relative flex h-1 w-full items-center overflow-x-hidden rounded-xs bg-paper-3",
		className
	)}
	value={indeterminate ? null : value}
	{max}
	{...restProps}
>
	{#if indeterminate}
		<div
			data-slot="progress-indicator"
			class="h-full w-1/3 rounded-xs bg-primary animate-sweep"
		></div>
	{:else}
		<div
			data-slot="progress-indicator"
			class="size-full flex-1 rounded-xs bg-primary motion-fill"
			style="transform: translateX(-{100 - (100 * (value ?? 0)) / (max ?? 1)}%)"
		></div>
	{/if}
</ProgressPrimitive.Root>
