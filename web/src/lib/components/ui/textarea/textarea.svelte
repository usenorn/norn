<script lang="ts">
	import { cn, type WithElementRef, type WithoutChildren } from "$lib/utils.js";
	import type { HTMLTextareaAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		value = $bindable(),
		class: className,
		"data-slot": dataSlot = "textarea",
		...restProps
	}: WithoutChildren<WithElementRef<HTMLTextareaAttributes>> = $props();
</script>

<textarea
	bind:this={ref}
	data-slot={dataSlot}
	class={cn(
		"flex field-sizing-content min-h-18 w-full resize-y rounded-none border-0 border-b border-input bg-transparent px-1 py-1.5 text-md leading-normal text-ink-900 outline-none transition-colors duration-110 ease-out placeholder:text-muted-foreground hover:border-ink-400 hover:bg-accent focus-visible:border-ring focus-visible:bg-accent focus-visible:rule-under aria-invalid:border-destructive aria-invalid:[--rule-under-color:var(--destructive)] disabled:pointer-events-none disabled:border-dashed disabled:text-ink-300",
		className
	)}
	bind:value
	{...restProps}></textarea>
