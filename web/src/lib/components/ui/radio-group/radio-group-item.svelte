<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const radioGroupItemVariants = tv({
		base: "group/radio-group-item peer relative shrink-0 border outline-none motion-control focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50",
		variants: {
			variant: {
				dot: "flex size-4 aspect-square rounded-full border-input after:absolute after:-inset-x-3 after:-inset-y-2 data-checked:border-primary data-checked:bg-primary data-checked:text-primary-foreground dark:bg-input/30 dark:data-checked:bg-primary",
				chip: "inline-flex items-center rounded-md border-line-default px-2 py-1 text-xs text-muted-foreground hover:text-ink-900 data-checked:border-primary data-checked:bg-primary data-checked:text-primary-foreground",
			},
		},
		defaultVariants: {
			variant: "dot",
		},
	});

	export type RadioGroupItemVariant = VariantProps<typeof radioGroupItemVariants>["variant"];
</script>

<script lang="ts">
	import { RadioGroup as RadioGroupPrimitive } from "bits-ui";
	import CircleIcon from "@lucide/svelte/icons/circle";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		variant = "dot",
		children: label,
		...restProps
	}: RadioGroupPrimitive.ItemProps & { variant?: RadioGroupItemVariant } = $props();
</script>

<RadioGroupPrimitive.Item
	bind:ref
	data-slot="radio-group-item"
	class={cn(radioGroupItemVariants({ variant }), className)}
	{...restProps}
>
	{#snippet children({ checked })}
		{#if variant === "chip"}
			{@render label?.({ checked })}
		{:else}
			<div data-slot="radio-group-indicator" class="flex size-4 items-center justify-center">
				{#if checked}
					<CircleIcon
						class="absolute top-1/2 left-1/2 size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-foreground"
					/>
				{/if}
			</div>
		{/if}
	{/snippet}
</RadioGroupPrimitive.Item>
