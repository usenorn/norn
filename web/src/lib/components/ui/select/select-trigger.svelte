<script lang="ts">
	import { Select as SelectPrimitive } from "bits-ui";
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import { cn, type WithoutChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		children,
		size = "default",
		...restProps
	}: WithoutChild<SelectPrimitive.TriggerProps> & {
		size?: "sm" | "default";
	} = $props();
</script>

<SelectPrimitive.Trigger
	bind:ref
	data-slot="select-trigger"
	data-size={size}
	class={cn(
		"flex w-full items-center justify-between gap-1.5 rounded-none border-0 border-b border-input bg-transparent px-1 text-md whitespace-nowrap text-ink-900 outline-none motion-control select-none hover:border-ink-400 hover:bg-accent focus-visible:border-ring focus-visible:bg-accent focus-visible:rule-under data-[size=default]:h-control-md data-[size=sm]:h-control-sm data-placeholder:text-muted-foreground aria-invalid:border-destructive disabled:pointer-events-none disabled:border-dashed disabled:text-ink-300 *:data-[slot=select-value]:line-clamp-1 *:data-[slot=select-value]:flex *:data-[slot=select-value]:items-center *:data-[slot=select-value]:gap-1.5 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-icon-row",
		className
	)}
	{...restProps}
>
	{@render children?.()}
	<ChevronDownIcon class="pointer-events-none size-icon-row text-muted-foreground" />
</SelectPrimitive.Trigger>
