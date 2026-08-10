<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const dropdownMenuContentVariants = tv({
		base: "notch min-w-46 text-popover-foreground data-open:animate-pop data-closed:animate-dismiss z-50 overflow-x-hidden overflow-y-auto outline-none data-closed:overflow-hidden",
		variants: {
			width: {
				anchor: "w-(--bits-dropdown-menu-anchor-width)",
				menu: "w-72 max-w-[calc(100vw---spacing(8))]",
			},
		},
		defaultVariants: {
			width: "anchor",
		},
	});

	export type DropdownMenuContentWidth = VariantProps<typeof dropdownMenuContentVariants>["width"];
</script>

<script lang="ts">
	import { DropdownMenu as DropdownMenuPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";
	import DropdownMenuPortal from "./dropdown-menu-portal.svelte";
	import type { ComponentProps } from "svelte";

	let {
		ref = $bindable(null),
		sideOffset = 4,
		align = "start",
		width = "anchor",
		portalProps,
		class: className,
		...restProps
	}: DropdownMenuPrimitive.ContentProps & {
		width?: DropdownMenuContentWidth;
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof DropdownMenuPortal>>;
	} = $props();
</script>

<DropdownMenuPortal {...portalProps}>
	<DropdownMenuPrimitive.Content
		bind:ref
		data-slot="dropdown-menu-content"
		{sideOffset}
		{align}
		class={cn(dropdownMenuContentVariants({ width }), className)}
		{...restProps}
	/>
</DropdownMenuPortal>
