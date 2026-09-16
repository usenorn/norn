<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const dialogOverlayVariants = tv({
		base: "bg-surface-scrim data-open:animate-scrim data-closed:animate-dismiss fixed inset-0 isolate z-50",
		variants: {
			blur: {
				true: "supports-backdrop-filter:backdrop-blur-[6px]",
				false: "",
			},
		},
		defaultVariants: {
			blur: true,
		},
	});

	export type DialogOverlayBlur = VariantProps<typeof dialogOverlayVariants>["blur"];
</script>

<script lang="ts">
	import { Dialog as DialogPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		blur = true,
		...restProps
	}: DialogPrimitive.OverlayProps & { blur?: DialogOverlayBlur } = $props();
</script>

<DialogPrimitive.Overlay
	bind:ref
	data-slot="dialog-overlay"
	class={cn(dialogOverlayVariants({ blur }), className)}
	{...restProps}
/>
