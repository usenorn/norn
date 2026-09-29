<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import type { CodeLink } from "$lib/source-control/source-control";
	import RepoChange from "./repo-change.svelte";
	import {
		changeStatLine,
		changeTotals,
		noChangesLine,
		validationLabel,
		type Execution,
		type ExecutionChangeSet,
	} from "./executions";

	let {
		execution,
		changeset,
		links,
		review,
		downloadOf,
	}: {
		execution: Execution;
		changeset?: ExecutionChangeSet;
		links: CodeLink[];
		review: string;
		downloadOf: (artifactId: string) => string;
	} = $props();

	const changes = $derived(changeset?.repositories ?? []);
	const totals = $derived(changeTotals(changes));
	const validation = $derived(changeset?.validation ?? []);

	const tones: Record<string, string> = {
		passed: "text-success",
		failed: "text-destructive",
		skipped: "text-muted-foreground",
	};
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="What changed">
	<Eyebrow rule>What changed</Eyebrow>

	{#if changes.length === 0}
		<p class="py-2 text-xs text-muted-foreground">{noChangesLine(execution)}</p>
	{:else}
		{#if changeset?.summary}
			<p class="max-w-prose text-sm leading-normal break-words text-ink-900 text-pretty">
				{changeset.summary}
			</p>
		{/if}

		<p class="font-mono text-xs text-muted-foreground">{changeStatLine(totals)}</p>

		<ul class="flex min-w-0 flex-col">
			{#each changes as change (change.repository)}
				<RepoChange
					{execution}
					{change}
					{links}
					{review}
					download={downloadOf(change.diffArtifactId ?? "")}
				/>
			{/each}
		</ul>
	{/if}

	{#if validation.length > 0}
		<ul class="flex min-w-0 flex-col gap-1 pt-1">
			{#each validation as check (check.check)}
				<li class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class={`text-xs ${tones[check.status]}`}>{validationLabel(check.status)}</span>
					<span class="min-w-0 text-xs break-words text-ink-900">{check.check}</span>
					{#if check.detail}
						<span class="min-w-0 text-2xs break-words text-muted-foreground">{check.detail}</span>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
