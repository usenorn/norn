<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import { cn } from "$lib/utils.js";
	import { priorityLabels, type TaskPriority } from "$lib/tasks/types";

	let {
		priority,
		withLabel = false,
		class: className,
	}: { priority: TaskPriority; withLabel?: boolean; class?: string } = $props();

	const bars = [
		{ x: 1.5, y: 9, height: 5 },
		{ x: 6, y: 5, height: 9 },
		{ x: 10.5, y: 1, height: 13 },
	];

	const filled = { urgent: 0, high: 3, medium: 2, low: 1, none: 0 };

	const tone = {
		urgent: "var(--priority-urgent)",
		high: "var(--priority-high)",
		medium: "var(--priority-medium)",
		low: "var(--priority-low)",
		none: "var(--priority-none)",
	};
</script>

{#snippet glyph()}
	{#if priority === "urgent"}
		<CircleAlert
			class={cn("size-icon-toolbar shrink-0 text-priority-urgent", className)}
			aria-label={priorityLabels[priority]}
		/>
	{:else}
		<svg
			viewBox="0 0 16 16"
			class={cn("size-icon-toolbar shrink-0", className)}
			role="img"
			aria-label={priorityLabels[priority]}
		>
			{#each bars as bar, index (index)}
				<rect
					x={bar.x}
					y={bar.y}
					width="3"
					height={bar.height}
					rx="1"
					fill={index < filled[priority] ? tone[priority] : "var(--priority-empty)"}
				/>
			{/each}
		</svg>
	{/if}
{/snippet}

{#if withLabel}
	<span class="inline-flex items-center gap-1.5 text-md text-foreground">
		{@render glyph()}
		{priorityLabels[priority]}
	</span>
{:else}
	{@render glyph()}
{/if}
