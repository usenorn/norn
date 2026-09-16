<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const commandListVariants = tv({
		base: "scroll-py-1 outline-none overflow-x-hidden overflow-y-auto",
		variants: {
			variant: {
				default: "max-h-60 p-0.75",
				palette: "max-h-[min(390px,calc(100dvh-16rem))] p-1.5",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type CommandListVariant = VariantProps<typeof commandListVariants>["variant"];
</script>

<script lang="ts">
	import { Command as CommandPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		variant = "default",
		...restProps
	}: CommandPrimitive.ListProps & { variant?: CommandListVariant } = $props();
</script>

<CommandPrimitive.List
	bind:ref
	data-slot="command-list"
	class={cn(commandListVariants({ variant }), className)}
	{...restProps}
/>
