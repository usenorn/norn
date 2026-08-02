<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const avatarVariants = tv({
		base: "group/avatar relative flex shrink-0 items-center justify-center overflow-hidden rounded-full font-mono font-medium select-none",
		variants: {
			size: {
				xs: "size-4.5 text-[9px]",
				sm: "size-6 text-2xs",
				default: "size-8 text-sm",
				lg: "size-10 text-base",
			},
			variant: {
				default: "bg-paper-3 text-ink-700",
				ghost: "border border-dashed border-line-strong bg-transparent text-ink-300",
			},
		},
		defaultVariants: {
			size: "default",
			variant: "default",
		},
	});

	export type AvatarSize = VariantProps<typeof avatarVariants>["size"];
	export type AvatarVariant = VariantProps<typeof avatarVariants>["variant"];
</script>

<script lang="ts">
	import { Avatar as AvatarPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		loadingStatus = $bindable("loading"),
		size = "default",
		variant = "default",
		class: className,
		...restProps
	}: AvatarPrimitive.RootProps & { size?: AvatarSize; variant?: AvatarVariant } = $props();
</script>

<AvatarPrimitive.Root
	bind:ref
	bind:loadingStatus
	data-slot="avatar"
	data-size={size}
	class={cn(avatarVariants({ size, variant }), className)}
	{...restProps}
/>
