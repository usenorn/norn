<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const alertActionVariants = tv({
		base: "flex flex-wrap items-center gap-2",
		variants: {
			placement: {
				corner: "absolute top-2.5 right-3",
				below: "mt-2 group-has-[>svg]/alert:col-start-2",
			},
		},
		defaultVariants: {
			placement: "corner",
		},
	});

	export type AlertActionPlacement = VariantProps<typeof alertActionVariants>["placement"];
</script>

<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		placement = "corner",
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> & {
		placement?: AlertActionPlacement;
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="alert-action"
	data-placement={placement}
	class={cn(alertActionVariants({ placement }), className)}
	{...restProps}
>
	{@render children?.()}
</div>
