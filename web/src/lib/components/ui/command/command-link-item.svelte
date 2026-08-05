<script lang="ts">
	import { Command as CommandPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: CommandPrimitive.LinkItemProps = $props();
</script>

<CommandPrimitive.LinkItem
	bind:ref
	data-slot="command-item"
	class={cn(
		"group/command-item relative flex h-7 cursor-default items-center gap-2 rounded-sm px-1.5 text-md text-foreground outline-hidden select-none data-selected:bg-accent data-selected:text-ink-900 aria-selected:bg-accent aria-selected:text-ink-900 data-[disabled=true]:pointer-events-none data-[disabled=true]:text-ink-300 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-icon-row",
		className
	)}
	{...restProps}
/>
