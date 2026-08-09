<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const textareaVariants = tv({
		base: "flex field-sizing-content min-h-18 w-full rounded-none border-0 border-b bg-transparent px-1 py-1.5 text-md leading-normal text-ink-900 outline-none motion-control placeholder:text-muted-foreground disabled:pointer-events-none disabled:border-dashed disabled:text-ink-300",
		variants: {
			variant: {
				default:
					"resize-y border-input hover:border-ink-400 hover:bg-accent focus-visible:border-ring focus-visible:bg-accent focus-visible:rule-under aria-invalid:border-destructive aria-invalid:[--rule-under-color:var(--destructive)]",
				seamless: "resize-none border-transparent",
			},
		},
		defaultVariants: { variant: "default" },
	});

	export type TextareaVariant = VariantProps<typeof textareaVariants>["variant"];
</script>

<script lang="ts">
	import { cn, type WithElementRef, type WithoutChildren } from "$lib/utils.js";
	import type { HTMLTextareaAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		value = $bindable(),
		variant = "default",
		class: className,
		"data-slot": dataSlot = "textarea",
		...restProps
	}: WithoutChildren<WithElementRef<HTMLTextareaAttributes>> & {
		variant?: TextareaVariant;
	} = $props();
</script>

<textarea
	bind:this={ref}
	data-slot={dataSlot}
	class={cn(textareaVariants({ variant }), className)}
	bind:value
	{...restProps}></textarea>
