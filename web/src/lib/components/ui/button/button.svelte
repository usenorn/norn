<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAnchorAttributes, HTMLButtonAttributes } from "svelte/elements";

	export const buttonVariants = tv({
		base: "group/button relative inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md border border-transparent font-medium tracking-snug whitespace-nowrap transition-colors duration-110 ease-out outline-none select-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring active:translate-y-px disabled:pointer-events-none disabled:opacity-40 disabled:shadow-none aria-disabled:pointer-events-none aria-disabled:opacity-40 aria-disabled:shadow-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-icon-row",
		variants: {
			variant: {
				default:
					"keycap bg-primary text-primary-foreground [--keycap-lip:var(--primary-active)] hover:bg-primary-hover active:bg-primary-active active:shadow-none",
				outline:
					"keycap border-line-strong bg-card text-foreground [--keycap-lip:var(--paper-0)] hover:border-ink-400 hover:bg-paper-2 active:bg-paper-0 active:shadow-none",
				secondary:
					"keycap border-line-strong bg-card text-foreground [--keycap-lip:var(--paper-0)] hover:border-ink-400 hover:bg-paper-2 active:bg-paper-0 active:shadow-none",
				subtle:
					"keycap bg-paper-2 text-foreground [--keycap-lip:var(--paper-0)] hover:bg-paper-3 active:bg-paper-1 active:shadow-none",
				ghost:
					"text-ink-600 hover:bg-accent hover:text-ink-900 active:bg-surface-active aria-expanded:bg-accent aria-expanded:text-ink-900",
				destructive:
					"keycap bg-destructive text-primary-foreground [--keycap-lip:var(--danger-active)] hover:bg-danger-hover active:shadow-none",
				link: "text-link underline-offset-2 hover:text-link-hover hover:underline",
				chip: "border-line-default bg-paper-2 font-mono text-ink-600 hover:bg-accent hover:text-ink-900",
			},
			size: {
				default: "h-control-md px-3 text-md",
				chip: "h-5 gap-1 rounded-xs px-1.5 text-xs",
				xs: "h-control-xs gap-1 rounded-sm px-1.5 text-2xs [&_svg:not([class*='size-'])]:size-3",
				sm: "h-control-sm gap-1 px-2 text-sm",
				lg: "h-control-lg px-4 text-base",
				icon: "size-control-md rounded-sm",
				"icon-xs": "size-control-xs rounded-sm [&_svg:not([class*='size-'])]:size-3",
				"icon-sm": "size-control-sm rounded-sm",
				"icon-lg": "size-control-lg rounded-sm",
				touch: "size-11 rounded-md [&_svg:not([class*='size-'])]:size-5",
			},
		},
		defaultVariants: {
			variant: "default",
			size: "default",
		},
	});

	export type ButtonVariant = VariantProps<typeof buttonVariants>["variant"];
	export type ButtonSize = VariantProps<typeof buttonVariants>["size"];

	export type ButtonProps = WithElementRef<HTMLButtonAttributes> &
		WithElementRef<HTMLAnchorAttributes> & {
			variant?: ButtonVariant;
			size?: ButtonSize;
		};
</script>

<script lang="ts">
	let {
		class: className,
		variant = "default",
		size = "default",
		ref = $bindable(null),
		href = undefined,
		type = "button",
		disabled,
		children,
		...restProps
	}: ButtonProps = $props();
</script>

{#if href}
	<a
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		href={disabled ? undefined : href}
		aria-disabled={disabled}
		role={disabled ? "link" : undefined}
		tabindex={disabled ? -1 : undefined}
		{...restProps}
	>
		{@render children?.()}
	</a>
{:else}
	<button
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		{type}
		{disabled}
		{...restProps}
	>
		{@render children?.()}
	</button>
{/if}
