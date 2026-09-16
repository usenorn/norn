<script lang="ts" module>
	import { tv } from "tailwind-variants";

	export const fieldTrigger = tv({
		base: "-ml-1.75 flex min-h-6 min-w-0 flex-1 items-center gap-1.75 rounded-sm px-1.75 py-0.5 text-left text-md text-ink-900 motion-control hover:bg-accent aria-expanded:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
	});
</script>

<script lang="ts">
	import type { Snippet } from "svelte";
	import PropertyPicker, { type PickerOption } from "$lib/issues/property-picker.svelte";

	let {
		label,
		options,
		placeholder,
		editable = true,
		open = $bindable(false),
		closeOnPick = true,
		empty = "No matches",
		onpick,
		control,
		glyph,
		value,
	}: {
		label: string;
		options?: PickerOption[];
		placeholder?: string;
		editable?: boolean;
		open?: boolean;
		closeOnPick?: boolean;
		empty?: string;
		onpick?: (value: string) => void;
		control?: Snippet;
		glyph?: Snippet;
		value: Snippet;
	} = $props();
</script>

<div class="relative flex min-h-7 items-center gap-1.5">
	<span
		class="w-19.5 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
	>
		{label}
	</span>
	{#if control}
		{@render control()}
	{:else if editable && options && placeholder !== undefined && onpick}
		<PropertyPicker
			{options}
			{placeholder}
			{empty}
			{closeOnPick}
			bind:open
			{onpick}
		>
			{#snippet trigger(props)}
				<button
					{...props}
					type="button"
					aria-label="{label}: change"
					class={fieldTrigger()}
				>
					{@render glyph?.()}
					{@render value()}
				</button>
			{/snippet}
		</PropertyPicker>
	{:else}
		<span class="flex min-h-6 min-w-0 flex-1 items-center gap-1.75 py-0.5 text-md text-ink-900">
			{@render glyph?.()}
			{@render value()}
		</span>
	{/if}
</div>
