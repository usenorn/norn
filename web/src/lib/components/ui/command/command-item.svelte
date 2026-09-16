<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const commandItemVariants = tv({
		base: "group/command-item relative flex cursor-default items-center text-md text-foreground outline-hidden select-none data-selected:bg-accent data-selected:text-ink-900 data-[disabled=true]:pointer-events-none data-[disabled=true]:text-ink-300 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-icon-row",
		variants: {
			variant: {
				default: "h-7 gap-2 rounded-sm px-1.5",
				palette:
					"h-7.5 gap-2.5 rounded-xs px-2 transition-none data-selected:shadow-[inset_2px_0_0_var(--primary)]",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type CommandItemVariant = VariantProps<typeof commandItemVariants>["variant"];
</script>

<script lang="ts">
	import { Command as CommandPrimitive } from "bits-ui";
	import CheckIcon from '@lucide/svelte/icons/check';
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		variant = "default",
		children,
		...restProps
	}: CommandPrimitive.ItemProps & { variant?: CommandItemVariant } = $props();
</script>

<CommandPrimitive.Item
	bind:ref
	data-slot="command-item"
	class={cn(commandItemVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
	<CheckIcon class="cn-command-item-indicator ml-auto opacity-0 group-has-[[data-slot=command-shortcut]]/command-item:hidden group-data-[checked=true]/command-item:opacity-100" />
</CommandPrimitive.Item>
