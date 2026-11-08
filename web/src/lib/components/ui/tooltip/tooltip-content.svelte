<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const tooltipContentVariants = tv({
		base: "notch text-md text-popover-foreground data-[state=delayed-open]:animate-pop data-[state=instant-open]:animate-pop data-closed:animate-dismiss z-50 w-fit max-w-xs origin-(--bits-tooltip-content-transform-origin) outline-hidden",
		variants: {
			size: {
				flush: "",
				body: "p-3",
			},
		},
		defaultVariants: {
			size: "body",
		},
	});

	export type TooltipContentSize = VariantProps<typeof tooltipContentVariants>["size"];
</script>

<script lang="ts">
	import { Tooltip as TooltipPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "#lib/utils.js";
	import TooltipPortal from "./tooltip-portal.svelte";
	import type { ComponentProps } from "svelte";

	let {
		ref = $bindable(null),
		class: className,
		size = "body",
		sideOffset = 4,
		side = "top",
		portalProps,
		...restProps
	}: TooltipPrimitive.ContentProps & {
		size?: TooltipContentSize;
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof TooltipPortal>>;
	} = $props();
</script>

<TooltipPortal {...portalProps}>
	<TooltipPrimitive.Content
		bind:ref
		data-slot="tooltip-content"
		{sideOffset}
		{side}
		class={cn(tooltipContentVariants({ size }), className)}
		{...restProps}
	/>
</TooltipPortal>
