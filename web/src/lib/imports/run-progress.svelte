<script lang="ts">
	import StepList, { type Step, type StepState } from "$lib/components/norn/step-list.svelte";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { counted, statusLabel, type ImportRun, type ImportStatus } from "./imports";

	let { run, busy = false }: { run: ImportRun; busy?: boolean } = $props();

	const order: ImportStatus[] = ["draft", "staging", "staged", "mapped", "executing", "imported"];

	const stages: { label: string; at: ImportStatus }[] = [
		{ label: "connect", at: "draft" },
		{ label: "read", at: "staging" },
		{ label: "decide", at: "staged" },
		{ label: "check", at: "mapped" },
		{ label: "import", at: "executing" },
		{ label: "done", at: "imported" },
	];

	function reached(status: ImportStatus): number {
		switch (status) {
			case "reverting":
			case "reverted":
				return order.length - 1;
			case "failed":
				return order.indexOf("executing");
			default:
				return Math.max(order.indexOf(status), 0);
		}
	}

	const steps = $derived<Step[]>(
		stages.map((stage, index) => {
			const at = reached(run.status);
			const state: StepState = index < at ? "done" : index === at ? "active" : "waiting";

			return { label: stage.label, state };
		})
	);
</script>

<div class="flex flex-col gap-3 rounded-lg border border-line-subtle bg-paper-0 p-4">
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<span class="text-sm font-medium tracking-snug text-ink-900">{statusLabel(run.status)}</span>
		<span class="font-mono text-xs text-muted-foreground tabular-nums">
			{counted(run.staged, "read", "read")} · {counted(run.processed, "handled", "handled")}
		</span>
	</div>

	{#if busy}
		<Progress indeterminate aria-label="{statusLabel(run.status)}, still going" />
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			The source is asked for a page at a time and is never asked how much it holds, so there is no
			total to count towards. The numbers above only go up.
		</p>
	{/if}

	<StepList {steps} />
</div>
