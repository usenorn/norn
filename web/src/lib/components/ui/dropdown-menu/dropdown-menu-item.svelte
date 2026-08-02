<script lang="ts">
	import { DropdownMenu as DropdownMenuPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		inset,
		variant = "default",
		...restProps
	}: DropdownMenuPrimitive.ItemProps & {
		inset?: boolean;
		variant?: "default" | "destructive";
	} = $props();
</script>

<DropdownMenuPrimitive.Item
	bind:ref
	data-slot="dropdown-menu-item"
	data-inset={inset}
	data-variant={variant}
	class={cn(
		"group/dropdown-menu-item relative flex h-6.75 cursor-default items-center gap-2 rounded-xs px-2 text-md text-foreground outline-hidden transition-none select-none focus:rule-inset focus:bg-accent focus:text-ink-900 data-inset:pl-8 data-[variant=destructive]:text-destructive data-[variant=destructive]:focus:bg-danger-quiet data-[variant=destructive]:focus:text-destructive data-[variant=destructive]:focus:[--rule-lead-color:var(--destructive)] data-[variant=destructive]:*:[svg]:text-destructive data-disabled:pointer-events-none data-disabled:text-ink-300 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-icon-row",
		className
	)}
	{...restProps}
/>
