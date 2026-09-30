<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const popoverContentVariants = tv({
		base: "notch text-md text-popover-foreground data-open:animate-pop data-closed:animate-dismiss z-50 w-72 origin-(--transform-origin) outline-hidden",
		variants: {
			size: {
				flush: "",
				body: "p-3",
			},
		},
		defaultVariants: {
			size: "flush",
		},
	});

	export type PopoverContentSize = VariantProps<typeof popoverContentVariants>["size"];
</script>

<script lang="ts">
	import { Popover as PopoverPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";
	import PopoverPortal from "./popover-portal.svelte";
	import type { ComponentProps } from "svelte";

	let {
		ref = $bindable(null),
		class: className,
		size = "flush",
		sideOffset = 4,
		align = "center",
		portalProps,
		...restProps
	}: PopoverPrimitive.ContentProps & {
		size?: PopoverContentSize;
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof PopoverPortal>>;
	} = $props();
</script>

<PopoverPortal {...portalProps}>
	<PopoverPrimitive.Content
		bind:ref
		data-slot="popover-content"
		{sideOffset}
		{align}
		class={cn(popoverContentVariants({ size }), className)}
		{...restProps}
	/>
</PopoverPortal>
