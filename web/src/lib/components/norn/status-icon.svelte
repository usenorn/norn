<script lang="ts">
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { cn } from "$lib/utils.js";
	import { categoryLabels, type StateCategory } from "$lib/team/states";

	let {
		category,
		name,
		withLabel = false,
		decorative = false,
		class: className,
	}: {
		category: StateCategory;
		name?: string;
		withLabel?: boolean;
		decorative?: boolean;
		class?: string;
	} = $props();

	const glyph = {
		not_started: CircleDashed,
		active: CircleDot,
		complete: CircleCheck,
		abandoned: CircleX,
	};

	const tone = {
		not_started: "text-status-not-started",
		active: "text-status-active",
		complete: "text-status-complete",
		abandoned: "text-status-abandoned",
	};

	const Glyph = $derived(glyph[category]);
	const label = $derived(name ?? categoryLabels[category]);
</script>

{#if withLabel}
	<span class={cn("inline-flex items-center gap-1.5 text-md text-foreground", className)}>
		<Glyph class="size-icon-row shrink-0 {tone[category]}" aria-hidden="true" />
		{label}
	</span>
{:else if decorative}
	<Glyph class={cn("size-icon-row shrink-0", tone[category], className)} aria-hidden="true" />
{:else}
	<Glyph class={cn("size-icon-row shrink-0", tone[category], className)} aria-label={label} />
{/if}
