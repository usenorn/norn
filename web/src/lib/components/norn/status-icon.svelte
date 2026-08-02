<script lang="ts">
	import Circle from "@lucide/svelte/icons/circle";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { cn } from "$lib/utils.js";
	import { statusLabels, type TaskStatus } from "$lib/tasks/types";

	let {
		status,
		withLabel = false,
		class: className,
	}: { status: TaskStatus; withLabel?: boolean; class?: string } = $props();

	const glyph = {
		backlog: CircleDashed,
		todo: Circle,
		started: CircleDot,
		review: CircleDot,
		done: CircleCheck,
		canceled: CircleX,
	};

	const tone = {
		backlog: "text-status-backlog",
		todo: "text-status-todo",
		started: "text-status-started",
		review: "text-status-review",
		done: "text-status-done",
		canceled: "text-status-canceled",
	};

	const Glyph = $derived(glyph[status]);
</script>

{#if withLabel}
	<span class={cn("inline-flex items-center gap-1.5 text-md text-foreground", className)}>
		<Glyph class="size-icon-row shrink-0 {tone[status]}" aria-hidden="true" />
		{statusLabels[status]}
	</span>
{:else}
	<Glyph
		class={cn("size-icon-row shrink-0", tone[status], className)}
		aria-label={statusLabels[status]}
	/>
{/if}
