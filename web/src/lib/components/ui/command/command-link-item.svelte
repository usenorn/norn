<script lang="ts">
	import { Command as CommandPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";
	import { commandItemVariants, type CommandItemVariant } from "./command-item.svelte";

	let {
		ref = $bindable(null),
		class: className,
		variant = "default",
		...restProps
	}: CommandPrimitive.LinkItemProps & { variant?: CommandItemVariant } = $props();
</script>

<CommandPrimitive.LinkItem
	bind:ref
	data-slot="command-item"
	class={cn(
		commandItemVariants({ variant }),
		"aria-selected:bg-accent aria-selected:text-ink-900",
		className
	)}
	{...restProps}
/>
