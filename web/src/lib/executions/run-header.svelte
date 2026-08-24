<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import RunState from "./run-state.svelte";
	import { elapsed } from "$lib/time";
	import {
		elapsedLabel,
		slotLine,
		standingLine,
		type Execution,
		type ExecutionRunner,
	} from "./executions";

	let {
		execution,
		runner,
		now,
	}: {
		execution: Execution;
		runner?: ExecutionRunner;
		now: string;
	} = $props();

	const from = $derived(execution.startedAt ?? execution.queuedAt);
	const to = $derived(execution.finishedAt ?? now);
	const ran = $derived(elapsed(from, to));
	const slots = $derived(slotLine(runner));

	const facts = $derived(
		[
			{ label: "Coding agent", value: execution.params.tool || "The machine's own" },
			{ label: "Model", value: execution.params.model || "The machine's own" },
			{ label: "Machine", value: execution.runnerName || "None yet" },
			{ label: "Folder", value: execution.codebaseName || "None yet" },
			{ label: "Slots", value: slots ?? "No machine yet" },
		].filter((fact) => fact.value !== "")
	);
</script>

<header class="flex min-w-0 flex-col gap-3">
	<div class="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
		<h1 class="font-mono text-lg text-ink-900">{execution.reference}</h1>
		<RunState state={execution.state} />
		<span class="font-mono text-xs text-muted-foreground">
			{elapsedLabel(execution.state)}
			{ran}
		</span>
		{#if execution.attempt > 1}
			<span class="font-mono text-2xs text-muted-foreground">attempt {execution.attempt}</span>
		{/if}
	</div>

	{#if execution.issueTitle}
		<p class="max-w-prose text-sm leading-normal break-words text-ink-900 text-pretty">
			{execution.issueTitle}
		</p>
	{/if}

	<p class="max-w-prose text-sm leading-normal break-words text-muted-foreground text-pretty">
		{standingLine(execution)}
	</p>

	<dl class="grid grid-cols-2 gap-x-4 gap-y-2.5 sm:grid-cols-3">
		{#each facts as fact (fact.label)}
			<div class="flex min-w-0 flex-col gap-0.5">
				<dt><Eyebrow>{fact.label}</Eyebrow></dt>
				<dd class="truncate text-xs text-ink-900" title={fact.value}>{fact.value}</dd>
			</div>
		{/each}
	</dl>
</header>
