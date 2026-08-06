<script lang="ts">
	import * as Table from "$lib/components/ui/table/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import {
		counted,
		importFailure,
		outcomeLabel,
		phaseLabel,
		resourceLabel,
		revertible,
		type ImportFailure,
		type ImportReport,
		type ImportResource,
		type ImportRun,
	} from "./imports";

	let {
		workspaceId,
		run,
		report,
		timezone,
		onadvanced,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		report: ImportReport;
		timezone: string;
		onadvanced: () => Promise<void>;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	let confirming = $state(false);
	let reverting = $state(false);

	const madeByResource = $derived(
		report.created.reduce<Map<ImportResource, number>>((counts, entry) => {
			counts.set(entry.resource, (counts.get(entry.resource) ?? 0) + 1);

			return counts;
		}, new Map())
	);
	const stillHere = $derived(report.created.filter((entry) => !entry.revertedAt).length);
	const takenBack = $derived(report.created.filter((entry) => entry.revertedAt).length);
	const kept = $derived(
		report.created.filter(
			(entry) =>
				entry.revertOutcome === "retained" ||
				entry.revertOutcome === "skipped_modified" ||
				entry.revertOutcome === "skipped_in_use"
		).length
	);

	async function revert() {
		reverting = true;
		onfailure(null);

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/imports/{importRunId}/revert", {
				params: { path: { workspaceId, importRunId: run.id } },
			});

			if (error) {
				onfailure(importFailure(error));

				return;
			}

			confirming = false;

			await onadvanced();
		} catch {
			onfailure({ kind: "unavailable" });
		} finally {
			reverting = false;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<section class="flex flex-col gap-3">
		<div class="flex flex-col gap-1">
			<h2 class="text-md font-medium tracking-snug text-ink-900">What this import made</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Every record below was created by this run and is held against it, which is what makes
				taking the run back possible at all.
			</p>
		</div>

		{#if report.created.length === 0}
			<p class="text-sm text-muted-foreground">Nothing was created.</p>
		{:else}
			<ul class="flex flex-wrap gap-x-4 gap-y-1 rounded-md border border-line-subtle bg-paper-0 p-3">
				{#each [...madeByResource] as [resource, made] (resource)}
					<li class="text-sm text-ink-900">
						<span class="font-mono text-xs text-muted-foreground">{resourceLabel(resource)}</span>
						{made.toLocaleString("en-GB")}
					</li>
				{/each}
			</ul>

			{#if run.status === "reverted" || run.status === "reverting"}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					{counted(takenBack, "record has been removed", "records have been removed")}.
					{#if kept > 0}
						{counted(kept, "was kept", "were kept")} because somebody has touched
						{kept === 1 ? "it" : "them"} since the import.
					{/if}
					{#if stillHere > 0}
						{counted(stillHere, "is still here", "are still here")}.
					{/if}
				</p>
			{/if}

			{#if report.nextCreatedCursor}
				<p class="text-xs text-muted-foreground">
					This page shows the most recent {report.created.length}. The rest are still recorded
					against the run.
				</p>
			{/if}
		{/if}
	</section>

	<section class="flex flex-col gap-3">
		<div class="flex flex-col gap-1">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Line by line</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				What the run did with each record it handled, including what it could not carry across.
			</p>
		</div>

		{#if report.lines.length === 0}
			<p class="text-sm text-muted-foreground">Nothing recorded yet.</p>
		{:else}
			<Table.Root class="min-w-160">
				<Table.Header>
					<Table.Row>
						<Table.Head>Phase</Table.Head>
						<Table.Head>What</Table.Head>
						<Table.Head>Subject</Table.Head>
						<Table.Head>Outcome</Table.Head>
						<Table.Head>Why</Table.Head>
						<Table.Head>When</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each report.lines as line (line.id)}
						<Table.Row>
							<Table.Cell class="text-muted-foreground">{phaseLabel(line.phase)}</Table.Cell>
							<Table.Cell class="text-muted-foreground">{resourceLabel(line.resource)}</Table.Cell>
							<Table.Cell class="text-ink-900">
								{line.subject ?? line.externalId ?? "—"}
							</Table.Cell>
							<Table.Cell class="text-muted-foreground">{outcomeLabel(line.outcome)}</Table.Cell>
							<Table.Cell class="text-muted-foreground">
								{Object.values(line.detail ?? {}).join(" · ") || "—"}
							</Table.Cell>
							<Table.Cell class="text-muted-foreground">
								{onDateAndTime(line.recordedAt, timezone)}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>

			{#if report.nextLineCursor}
				<p class="text-xs text-muted-foreground">
					More lines were recorded than are shown here.
				</p>
			{/if}
		{/if}
	</section>

	{#if revertible(run.status)}
		<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
			<div class="flex flex-col gap-1">
				<h2 class="text-md font-medium tracking-snug text-ink-900">Take this import back</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					This removes what the import created, in the reverse of the order it created it. Anything
					a person has commented on, edited or moved since is kept and listed rather than removed.
					Nothing that was here before the import is touched. It cannot be undone: importing again
					means reading the source again.
				</p>
			</div>

			{#if confirming}
				<div class="flex flex-wrap items-center gap-2">
					<p class="text-sm text-ink-900">
						Remove the {counted(stillHere, "record", "records")} this import created?
					</p>
					<Button variant="destructive" size="sm" disabled={reverting} onclick={revert}>
						{reverting ? "Taking it back…" : "Yes, take it back"}
					</Button>
					<Button
						variant="ghost"
						size="sm"
						disabled={reverting}
						onclick={() => (confirming = false)}
					>
						Keep it
					</Button>
				</div>
			{:else}
				<div>
					<Button
						variant="secondary"
						size="sm"
						disabled={reverting}
						onclick={() => (confirming = true)}
					>
						Take this import back
					</Button>
				</div>
			{/if}
		</section>
	{/if}
</div>
