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
			tone: {
				neutral: "",
				cyan: "bg-avatar-cyan text-avatar-cyan-ink",
				blue: "bg-avatar-blue text-avatar-blue-ink",
				violet: "bg-avatar-violet text-avatar-violet-ink",
				orchid: "bg-avatar-orchid text-avatar-orchid-ink",
				magenta: "bg-avatar-magenta text-avatar-magenta-ink",
				rose: "bg-avatar-rose text-avatar-rose-ink",
				amber: "bg-avatar-amber text-avatar-amber-ink",
				green: "bg-avatar-green text-avatar-green-ink",
			},
		},
		defaultVariants: {
			size: "default",
			variant: "default",
			tone: "neutral",
		},
	});

	export type AvatarSize = VariantProps<typeof avatarVariants>["size"];
	export type AvatarVariant = VariantProps<typeof avatarVariants>["variant"];
	export type AvatarToneVariant = VariantProps<typeof avatarVariants>["tone"];
</script>

<script lang="ts">
	import { Avatar as AvatarPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		loadingStatus = $bindable("loading"),
		size = "default",
		variant = "default",
		tone = "neutral",
		class: className,
		...restProps
	}: AvatarPrimitive.RootProps & {
		size?: AvatarSize;
		variant?: AvatarVariant;
		tone?: AvatarToneVariant;
	} = $props();
</script>

<AvatarPrimitive.Root
	bind:ref
	bind:loadingStatus
	data-slot="avatar"
	data-size={size}
	class={cn(avatarVariants({ size, variant, tone }), className)}
	{...restProps}
/>
