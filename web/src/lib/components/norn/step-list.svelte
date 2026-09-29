<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";

	export type StepState = "done" | "active" | "waiting" | "stopped";
	export type Step = { label: string; state: StepState };

	export const stepListVariants = tv({
		base: "flex",
		variants: {
			orientation: {
				vertical: "flex-col gap-0.5",
				horizontal: "flex-wrap items-center gap-x-3 gap-y-1",
			},
		},
		defaultVariants: { orientation: "vertical" },
	});

	export type StepListOrientation = VariantProps<typeof stepListVariants>["orientation"];
</script>

<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { cn } from "$lib/utils.js";

	let {
		steps,
		orientation = "vertical",
		class: className,
	}: { steps: Step[]; orientation?: StepListOrientation; class?: string } = $props();

	const glyph = { done: Check, active: CircleDot, waiting: CircleDashed, stopped: CircleX };
	const glyphTone = {
		done: "text-success",
		active: "text-ink-600",
		waiting: "text-muted-foreground",
		stopped: "text-destructive",
	};
	const labelTone = {
		done: "text-ink-600",
		active: "text-ink-900",
		waiting: "text-muted-foreground",
		stopped: "text-destructive",
	};
</script>

<ol class={cn(stepListVariants({ orientation }), className)}>
	{#each steps as step (step.label)}
		{@const Glyph = glyph[step.state]}
		<li class="flex h-6.5 items-center gap-2" aria-current={step.state === "active" ? "step" : undefined}>
			<Glyph class="size-icon-row shrink-0 {glyphTone[step.state]}" aria-hidden="true" />
			<span class="font-mono text-xs {labelTone[step.state]}">{step.label}</span>
		</li>
	{/each}
</ol>
