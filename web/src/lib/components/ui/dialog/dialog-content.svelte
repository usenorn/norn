<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const dialogContentVariants = tv({
		base: "notch grid max-w-[calc(100%-3rem)] gap-6 p-6 text-md text-popover-foreground sm:max-w-md data-open:animate-lift data-closed:animate-dismiss fixed top-[12vh] left-1/2 z-50 w-full -translate-x-1/2 outline-none",
		variants: {
			variant: {
				default: "",
				scrollable:
					"top-[max(1rem,env(safe-area-inset-top))] max-h-[calc(100dvh-2rem-env(safe-area-inset-top)-env(safe-area-inset-bottom))] overflow-y-auto overscroll-contain pb-[calc(1.5rem+env(safe-area-inset-bottom))] sm:top-[12vh] sm:max-h-[76dvh] sm:pb-6",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type DialogContentVariant = VariantProps<typeof dialogContentVariants>["variant"];
</script>

<script lang="ts">
	import { Dialog as DialogPrimitive } from "bits-ui";
	import XIcon from "@lucide/svelte/icons/x";
	import { Button } from "$lib/components/ui/button/index.js";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";
	import * as Dialog from "./index.js";
	import DialogPortal from "./dialog-portal.svelte";
	import type { Snippet } from "svelte";
	import type { ComponentProps } from "svelte";

	let {
		ref = $bindable(null),
		class: className,
		portalProps,
		children,
		showCloseButton = true,
		variant = "default",
		...restProps
	}: WithoutChildrenOrChild<DialogPrimitive.ContentProps> & {
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof DialogPortal>>;
		children: Snippet;
		showCloseButton?: boolean;
		variant?: DialogContentVariant;
	} = $props();
</script>

<DialogPortal {...portalProps}>
	<Dialog.Overlay />
	<DialogPrimitive.Content
		bind:ref
		data-slot="dialog-content"
		class={cn(
			dialogContentVariants({ variant }),
			className
		)}
		{...restProps}
	>
		{@render children?.()}
		{#if showCloseButton}
			<DialogPrimitive.Close data-slot="dialog-close">
				{#snippet child({ props })}
					<Button variant="ghost" class="absolute top-4 right-4" size="icon-sm" {...props}>
						<XIcon />
						<span class="sr-only">Close</span>
					</Button>
				{/snippet}
			</DialogPrimitive.Close>
		{/if}
	</DialogPrimitive.Content>
</DialogPortal>
