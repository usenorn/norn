<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const alertVariants = tv({
		base: "group/alert rule-lead relative grid w-full gap-0.5 rounded-lg border border-line-default bg-card px-3 py-2.5 text-left text-md has-data-[placement=corner]:relative has-data-[placement=corner]:pr-18 has-[>svg]:grid-cols-[auto_1fr] has-[>svg]:gap-x-2.5 *:[svg]:row-span-2 *:[svg]:mt-px *:[svg:not([class*='size-'])]:size-icon-row",
		variants: {
			variant: {
				default: "[--rule-lead-color:var(--ink-400)] *:[svg]:text-ink-400",
				muted: "[--rule-lead-color:var(--ink-400)] *:[svg]:text-muted-foreground",
				destructive: "[--rule-lead-color:var(--destructive)] *:[svg]:text-destructive",
				warning: "[--rule-lead-color:var(--warning)] *:[svg]:text-warning",
				success: "[--rule-lead-color:var(--success)] *:[svg]:text-success",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type AlertVariant = VariantProps<typeof alertVariants>["variant"];
</script>

<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		variant = "default",
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> & {
		variant?: AlertVariant;
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="alert"
	role="alert"
	class={cn(alertVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
</div>
