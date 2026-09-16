<script lang="ts" module>
	import { type VariantProps, tv } from "tailwind-variants";

	export const commandInputVariants = tv({
		base: "flex flex-none items-center border-b border-line-subtle",
		variants: {
			variant: {
				default: "h-8 gap-1.5 px-2.25",
				palette: "h-11 gap-2.5 px-3",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type CommandInputVariant = VariantProps<typeof commandInputVariants>["variant"];
</script>

<script lang="ts">
	import { Command as CommandPrimitive } from "bits-ui";
	import SearchIcon from "@lucide/svelte/icons/search";
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		value = $bindable(""),
		variant = "default",
		leading,
		children,
		...restProps
	}: CommandPrimitive.InputProps & {
		variant?: CommandInputVariant;
		leading?: Snippet;
	} = $props();
</script>

<div data-slot="command-input-wrapper" class={commandInputVariants({ variant })}>
	{#if leading}
		{@render leading()}
	{:else}
		<SearchIcon class="size-3.25 shrink-0 text-muted-foreground" aria-hidden="true" />
	{/if}
	<CommandPrimitive.Input
		bind:ref
		bind:value
		data-slot="command-input"
		class={cn(
			"min-w-0 flex-1 bg-transparent text-md text-ink-900 outline-hidden placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50",
			className
		)}
		{...restProps}
	/>
	{@render children?.()}
</div>
