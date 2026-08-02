<script lang="ts" module>
	export type StepState = "done" | "active" | "waiting";
	export type Step = { label: string; state: StepState };
</script>

<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import { cn } from "$lib/utils.js";

	let { steps, class: className }: { steps: Step[]; class?: string } = $props();

	const glyph = { done: Check, active: CircleDot, waiting: CircleDashed };
	const glyphTone = {
		done: "text-success",
		active: "text-ink-600",
		waiting: "text-muted-foreground",
	};
	const labelTone = {
		done: "text-ink-600",
		active: "text-ink-900",
		waiting: "text-muted-foreground",
	};
</script>

<ol class={cn("flex flex-col gap-0.5", className)}>
	{#each steps as step (step.label)}
		{@const Glyph = glyph[step.state]}
		<li class="flex h-6.5 items-center gap-2">
			<Glyph class="size-icon-row shrink-0 {glyphTone[step.state]}" aria-hidden="true" />
			<span class="font-mono text-xs {labelTone[step.state]}">{step.label}</span>
		</li>
	{/each}
</ol>
