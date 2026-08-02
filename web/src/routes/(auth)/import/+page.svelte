<script lang="ts">
	import type { Component } from "svelte";
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Check from "@lucide/svelte/icons/check";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Folder from "@lucide/svelte/icons/folder";
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import Inbox from "@lucide/svelte/icons/inbox";
	import Layers from "@lucide/svelte/icons/layers";
	import Minus from "@lucide/svelte/icons/minus";
	import TableIcon from "@lucide/svelte/icons/table";
	import Target from "@lucide/svelte/icons/target";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Table from "$lib/components/ui/table/index.js";
	import CountGrid from "$lib/components/import/count-grid.svelte";
	import Diagnostics from "$lib/components/norn/diagnostics.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import MappingTable from "$lib/components/import/mapping-table.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { connectSchema } from "$lib/import/connect-schema";
	import type { ImportSourceId, ImportStage } from "$lib/import/types";
	import { importPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const connectFormId = "import-connect-form";

	const sourceCatalog = {
		jira: { name: "Jira", how: "API token", icon: Layers },
		linear: { name: "Linear", how: "API key", icon: Target },
		github: { name: "GitHub Issues", how: "OAuth or token", icon: GitBranch },
		csv: { name: "CSV file", how: "Upload a file", icon: TableIcon },
	} satisfies Record<ImportSourceId, { name: string; how: string; icon: Component }>;

	const sourceOrder: ImportSourceId[] = ["jira", "linear", "github", "csv"];

	const steps = ["Source", "Connect", "Map", "Preview", "Import"];

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? importPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let submitStage = $state<ImportStage | null>(null);
	let picked = $state<ImportSourceId | null>(null);

	const session = $derived({ ...data.session, ...preview?.session });
	const stage = $derived<ImportStage>(submitStage ?? session.stage);
	const source = $derived(picked ?? session.source);
	const sourceName = $derived(sourceCatalog[source].name);

	const form = superForm(defaults(zod4(connectSchema)), {
		id: connectFormId,
		SPA: true,
		validators: zod4Client(connectSchema),
		resetForm: false,
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const prefill = preview?.form;
		if (prefill) formData.update((current) => ({ ...current, ...prefill }), { taint: false });
	});

	const busy = $derived($submitting);

	const unresolvedFields = $derived(
		stage.kind === "map_fields"
			? stage.mapping.groups.flatMap((group) => group.rows).filter((row) => row.needsDecision)
			: []
	);
	const unmatchedPeople = $derived(
		stage.kind === "map_people" ? stage.matching.people.filter((person) => person.needsDecision) : []
	);

	function issueTotal(volumes: string[]) {
		return volumes
			.reduce((total, volume) => total + Number(volume.replace(/,/g, "")), 0)
			.toLocaleString("en-US");
	}

	const stepIndex = $derived.by(() => {
		switch (stage.kind) {
			case "choose_source":
				return 1;
			case "connect":
				return 2;
			case "map_fields":
			case "map_people":
				return 3;
			case "preview":
				return 4;
			case "running":
			case "finished":
				return 5;
		}
	});

	const title = $derived.by(() => {
		switch (stage.kind) {
			case "choose_source":
				return "Import from another tool";
			case "connect":
				return `Connect to ${sourceName}`;
			case "map_fields":
				return stage.unresolvedOnly ? "Three decisions left" : `Map ${sourceName} onto Norn`;
			case "map_people":
				return "Match people";
			case "preview":
				return "Preview";
			case "running":
				return stage.detached ? "Running in the background" : `Importing from ${sourceName}`;
			case "finished":
				switch (stage.outcome.kind) {
					case "complete":
						return `Imported ${stage.outcome.imported} issues`;
					case "with_skips":
						return `Imported ${stage.outcome.imported} of ${stage.outcome.total} issues`;
					case "failed":
						return `Import stopped after ${stage.outcome.stoppedAfter} issues`;
				}
		}
	});

	const lede = $derived.by(() => {
		switch (stage.kind) {
			case "choose_source":
				return "Issues, comments and history come across. Nothing is created until you approve a preview.";
			case "connect":
				return stage.failure
					? "The token was rejected. Nothing has been read or created."
					: `A read-only token is enough. Norn never writes back to ${sourceName}.`;
			case "map_fields":
				return stage.unresolvedOnly
					? "Everything else is mapped. These three have no obvious match — choose one, or skip those issues."
					: "This is the step imports usually get wrong, so it's worth a minute. Statuses and priorities carry the most weight.";
			case "map_people":
				return "Assignees only carry over when a person matches. Unmatched work arrives unassigned.";
			case "preview":
				return "This is exactly what will be created, and it can be rolled back for 24 hours.";
			case "running":
				return stage.detached
					? "The card in the corner follows you while it runs."
					: "Two projects done. You can leave this page.";
			case "finished":
				switch (stage.outcome.kind) {
					case "complete":
						return `${stage.outcome.landedIn} are ready.`;
					case "with_skips":
						return `${stage.outcome.skippedTotal} items were skipped. Every one has a reason and can be retried.`;
					case "failed":
						return `${sourceName} stopped answering. Nothing was deleted — resuming picks up at issue ${stage.outcome.resumeAt}.`;
				}
		}
	});

	const status = $derived.by(() => {
		if (stage.kind === "map_fields") {
			return stage.unresolvedOnly
				? { label: `${unresolvedFields.length} unmapped`, tone: "text-warning" }
				: {
						label: `${stage.mapping.fieldsMapped} of ${stage.mapping.fieldsFound} mapped`,
						tone: "text-muted-foreground",
					};
		}
		if (stage.kind === "map_people") {
			return {
				label: `${stage.matching.matched} of ${stage.matching.people.length} matched`,
				tone: "text-muted-foreground",
			};
		}
		if (stage.kind === "running") {
			return { label: `${stage.progress.percent}%`, tone: "text-muted-foreground" };
		}
		return null;
	});

	const mapSummary = $derived.by(() => {
		if (stage.kind === "map_people") return "Three of four people matched by email address.";
		if (stage.kind !== "map_fields") return null;
		return stage.unresolvedOnly
			? `${stage.mapping.affectedIssues} issues are affected by these three.`
			: `${stage.mapping.fieldsFound} fields found · ${stage.mapping.fieldsMapped} mapped automatically.`;
	});

	const mapNote = $derived.by(() => {
		if (stage.kind === "map_people") {
			return "Unmatched assignees arrive unassigned. Bulk-assign later from the list view.";
		}
		if (stage.kind !== "map_fields") return null;
		return stage.unresolvedOnly
			? "Skipping a value leaves those issues out of the import entirely."
			: "People are matched on the next step.";
	});

	const scope = [
		{ included: true, label: "Issues, descriptions and comments" },
		{ included: true, label: "Status, priority, labels and assignees" },
		{ included: true, label: "Attachments up to 25 MB" },
		{ included: false, label: "Sprints, boards and automation rules" },
	];

	const connectFixes = [
		"Create the token from the same account that can see the projects.",
		"Grant read:jira-work when you create it.",
		"If your site enforces SSO for API access, use a scoped app token instead.",
	];

	const resultNote = $derived.by(() => {
		if (stage.kind !== "finished") return null;
		switch (stage.outcome.kind) {
			case "complete":
				return `Every issue keeps a link back to its ${sourceName} key.`;
			case "with_skips":
				return `Every issue keeps its ${sourceName} key, so a skipped item is easy to find later.`;
			case "failed":
				return `Nothing was deleted. Resuming continues at issue ${stage.outcome.resumeAt} and skips anything already created.`;
		}
	});

	const cta = $derived.by(() => {
		switch (stage.kind) {
			case "choose_source":
				return { label: "Continue", form: null, href: null };
			case "connect":
				return {
					label: stage.failure ? "Try again" : busy ? "Connecting" : "Connect",
					form: connectFormId,
					href: null,
				};
			case "map_fields":
				return {
					label: stage.unresolvedOnly
						? "Continue"
						: `Review ${unresolvedFields.length} decisions`,
					form: null,
					href: null,
				};
			case "map_people":
				return { label: "Continue", form: null, href: null };
			case "preview":
				return { label: "Start import", form: null, href: null };
			case "running":
				return stage.detached
					? { label: "Open Norn", form: null, href: "/issues" }
					: { label: "Run in background", form: null, href: null };
			case "finished":
				switch (stage.outcome.kind) {
					case "complete":
					case "with_skips":
						return {
							label: `Open ${stage.outcome.primaryTeam}`,
							form: null,
							href: "/issues",
						};
					case "failed":
						return { label: "Resume import", form: null, href: null };
				}
		}
	});

	const secondary = $derived.by(() => {
		if (stage.kind === "map_fields" && stage.unresolvedOnly) return "Skip those issues";
		if (stage.kind === "running") return stage.detached ? "Show progress" : "Cancel import";
		if (stage.kind === "finished") {
			switch (stage.outcome.kind) {
				case "complete":
					return "Import report";
				case "with_skips":
					return `Retry ${stage.outcome.skippedTotal} items`;
				case "failed":
					return "Roll back everything";
			}
		}
		return null;
	});

	const showBack = $derived(
		stage.kind === "connect" ||
			stage.kind === "map_fields" ||
			stage.kind === "map_people" ||
			stage.kind === "preview"
	);

	const footnote = $derived.by(() => {
		switch (stage.kind) {
			case "choose_source":
				return { label: `${sourceName} selected`, tone: "text-muted-foreground" };
			case "connect":
				return stage.failure
					? { label: "Nothing was imported", tone: "text-muted-foreground" }
					: null;
			case "map_fields":
				return stage.unresolvedOnly
					? {
							label: `${unresolvedFields.length} decisions left · ${stage.mapping.affectedIssues} issues`,
							tone: "text-warning",
						}
					: {
							label: `${unresolvedFields.length} fields still need a decision`,
							tone: "text-warning",
						};
			case "map_people":
				return {
					label: `${unmatchedPeople.length} ${unmatchedPeople.length === 1 ? "person" : "people"} unmatched · ${issueTotal(unmatchedPeople.map((person) => person.volume))} issues`,
					tone: "text-muted-foreground",
				};
			case "preview":
				return { label: "Nothing has been created yet", tone: "text-muted-foreground" };
			case "running":
				return stage.detached
					? { label: `Running · ${stage.progress.percent}%`, tone: "text-muted-foreground" }
					: { label: "Safe to close this tab", tone: "text-muted-foreground" };
			case "finished":
				switch (stage.outcome.kind) {
					case "complete":
					case "with_skips":
						return {
							label: `Finished ${stage.outcome.finishedAt} · ${stage.outcome.duration}`,
							tone: "text-muted-foreground",
						};
					case "failed":
						return {
							label: `Stopped ${stage.outcome.stoppedAt} · ${stage.outcome.stoppedAfter} of ${stage.outcome.total}`,
							tone: "text-destructive",
						};
				}
		}
	});
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full max-w-260 flex-col gap-5">
	<ol class="flex flex-wrap items-center gap-x-1 gap-y-2 px-0.5">
		{#each steps as label, index (label)}
			{@const number = index + 1}
			{@const done = number < stepIndex}
			{@const current = number === stepIndex}
			<li class="inline-flex flex-none items-center gap-1.5">
				<span
					class="inline-flex size-4.5 items-center justify-center rounded-xs border font-mono text-2xs {current
						? 'border-primary bg-primary text-primary-foreground'
						: done
							? 'border-line-default text-ink-600'
							: 'border-line-default text-muted-foreground'}"
					aria-hidden="true"
				>
					{done ? "✓" : number}
				</span>
				<span
					class="font-mono text-2xs tracking-eyebrow whitespace-nowrap uppercase {current
						? 'text-ink-900'
						: done
							? 'text-ink-600'
							: 'text-muted-foreground'}"
				>
					{label}
					<span class="sr-only">{current ? "(current step)" : done ? "(done)" : "(to do)"}</span>
				</span>
			</li>
			{#if number < steps.length}
				<li class="mx-2 h-px min-w-3 flex-[1_1_16px] bg-line-subtle" aria-hidden="true"></li>
			{/if}
		{/each}
	</ol>

	<div class="notch w-full">
		<div class="flex flex-col">
			<div
				class="flex flex-wrap items-baseline justify-between gap-2 border-b border-line-subtle px-5 pt-5 pb-4 sm:px-5.5"
			>
				<div class="flex min-w-55 flex-1 flex-col gap-1.5">
					<h1 class="text-xl font-medium tracking-title text-ink-900">{title}</h1>
					<p class="max-w-[70ch] text-md leading-normal text-muted-foreground text-pretty">
						{lede}
					</p>
				</div>
				{#if status}
					<span class="font-mono text-xs whitespace-nowrap {status.tone}">{status.label}</span>
				{/if}
			</div>

			<div class="flex flex-col gap-4.5 px-5 py-5 sm:px-5.5">
				{#if stage.kind === "choose_source"}
					<ul class="flex flex-wrap gap-2.5">
						{#each sourceOrder as id (id)}
							{@const entry = sourceCatalog[id]}
							{@const SourceIcon = entry.icon}
							{@const selected = source === id}
							<li class="flex min-w-45 flex-[1_1_200px]">
								<button
									type="button"
									onclick={() => (picked = id)}
									aria-pressed={selected}
									class="flex w-full items-start gap-2.5 rounded-lg border p-3.5 text-left transition-colors duration-110 ease-out focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring {selected
										? 'border-ink-400 bg-surface-selected'
										: 'border-line-default bg-card hover:bg-accent'}"
								>
									<SourceIcon
										class="mt-px size-icon-toolbar shrink-0 {selected
											? 'text-ink-900'
											: 'text-muted-foreground'}"
										aria-hidden="true"
									/>
									<span class="flex min-w-0 flex-col gap-0.5">
										<span class="text-md font-medium text-ink-900">{entry.name}</span>
										<span class="font-mono text-xs text-muted-foreground">{entry.how}</span>
									</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}

				{#if stage.kind === "connect"}
					<div class="flex flex-wrap items-start gap-5">
						<form
							id={connectFormId}
							method="POST"
							use:enhance
							class="flex min-w-65 flex-[1_1_320px] flex-col gap-4"
						>
							<Form.Field {form} name="site">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>{sourceName} site</Form.Label>
										<Input
											{...props}
											type="url"
											inputmode="url"
											autocapitalize="none"
											spellcheck="false"
											placeholder="https://example.atlassian.net"
											disabled={busy}
											bind:value={$formData.site}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="email">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Account email</Form.Label>
										<Input
											{...props}
											type="email"
											inputmode="email"
											autocomplete="email"
											autocapitalize="none"
											spellcheck="false"
											placeholder="you@company.com"
											disabled={busy}
											bind:value={$formData.email}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="token">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>API token</Form.Label>
										<Input
											{...props}
											type="password"
											autocomplete="off"
											disabled={busy}
											aria-invalid={stage.failure ? "true" : undefined}
											bind:value={$formData.token}
										/>
									{/snippet}
								</Form.Control>
								{#if stage.failure}
									<p class="flex items-center gap-1.5 text-sm text-destructive" role="alert">
										<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
										Rejected by {sourceName}.
									</p>
								{:else}
									<Form.Description class="text-sm text-muted-foreground">
										Read-only is enough. Nothing is ever written back to {sourceName}.
									</Form.Description>
								{/if}
								<Form.FieldErrors />
							</Form.Field>
						</form>

						<div
							class="flex min-w-60 flex-[1_1_260px] flex-col gap-2 rounded-lg border border-line-default bg-card p-3.5"
						>
							<Eyebrow class="text-ink-600">What comes across</Eyebrow>
							<ul class="flex flex-col gap-2">
								{#each scope as entry (entry.label)}
									{@const ScopeIcon = entry.included ? Check : Minus}
									<li class="flex items-start gap-2">
										<ScopeIcon
											class="mt-px size-icon-row shrink-0 {entry.included
												? 'text-success'
												: 'text-muted-foreground'}"
											aria-hidden="true"
										/>
										<span
											class="flex-1 text-sm leading-normal {entry.included
												? 'text-ink-600'
												: 'text-muted-foreground'}"
										>
											{entry.label}
										</span>
									</li>
								{/each}
							</ul>
						</div>
					</div>

					{#if stage.failure}
						<Alert.Root variant="destructive">
							<CircleAlert aria-hidden="true" />
							<Alert.Title>{sourceName} rejected the token</Alert.Title>
							<Alert.Description>
								The credentials are valid but the token can't read issues. This is a scope problem,
								not a password problem.
							</Alert.Description>
						</Alert.Root>

						<div class="flex flex-wrap items-start gap-4">
							<Diagnostics
								label="Response"
								entries={stage.failure.diagnostics}
								keyWidth="w-20 sm:w-23"
								class="min-w-70 flex-[1_1_320px]"
							/>
							<div class="flex min-w-60 flex-[1_1_260px] flex-col gap-2">
								<Eyebrow class="text-ink-600">What to check</Eyebrow>
								<ul class="flex flex-col gap-2">
									{#each connectFixes as fix (fix)}
										<li class="flex items-start gap-2">
											<span
												class="mt-2 size-1 flex-none bg-muted-foreground"
												aria-hidden="true"
											></span>
											<span class="flex-1 text-md leading-normal text-ink-600">{fix}</span>
										</li>
									{/each}
								</ul>
							</div>
						</div>
					{/if}
				{/if}

				{#if stage.kind === "map_fields" || stage.kind === "map_people"}
					<div class="flex flex-col gap-3">
						<div class="flex flex-wrap items-center justify-between gap-2">
							<span class="text-md text-ink-600">{mapSummary}</span>
							{#if stage.kind === "map_fields" && stage.unresolvedOnly}
								<Button variant="ghost" size="sm">Show everything</Button>
							{/if}
						</div>

						{#if stage.kind === "map_fields"}
							<MappingTable
								columnLabel="In {sourceName}"
								sections={stage.mapping.groups.map((group) => ({
									name: group.name,
									rows: group.rows,
								}))}
							/>
						{:else}
							<MappingTable
								columnLabel="Person in {sourceName}"
								identity="person"
								sections={[{ name: null, rows: stage.matching.people }]}
							/>
						{/if}

						<p class="text-sm leading-normal text-muted-foreground text-pretty">{mapNote}</p>
					</div>
				{/if}

				{#if stage.kind === "preview"}
					<div class="flex flex-col gap-5">
						<CountGrid counts={stage.plan.counts} />

						<div class="flex flex-wrap items-start gap-5">
							<div class="flex min-w-65 flex-[1_1_300px] flex-col gap-2">
								<Eyebrow class="text-ink-600">Where it lands</Eyebrow>
								<ul class="flex flex-col">
									{#each stage.plan.destinations as destination (destination.label)}
										<li
											class="flex h-6.5 items-center gap-2 border-b border-line-subtle last:border-b-0"
										>
											<Folder class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
											<span class="flex-1 truncate text-md text-ink-600">{destination.label}</span>
											<span class="font-mono text-xs text-muted-foreground tabular-nums">
												{destination.count}
											</span>
										</li>
									{/each}
								</ul>
							</div>

							<div class="flex min-w-65 flex-[1_1_300px] flex-col gap-2">
								<Eyebrow class="text-ink-600">What won't come across</Eyebrow>
								<ul class="flex flex-col">
									{#each stage.plan.excluded as item (item)}
										<li class="flex items-start gap-2 py-1">
											<Minus
												class="mt-0.5 size-icon-row shrink-0 text-muted-foreground"
												aria-hidden="true"
											/>
											<span class="flex-1 text-md leading-normal text-ink-600">{item}</span>
										</li>
									{/each}
								</ul>
							</div>
						</div>

						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Nothing is created until you start the import. It can be rolled back for 24 hours
							afterwards.
						</p>
					</div>
				{/if}

				{#if stage.kind === "running" && !stage.detached}
					<div class="flex flex-col gap-4" aria-live="polite">
						<div class="flex flex-col gap-2">
							<div class="flex flex-wrap items-baseline justify-between gap-3">
								<span class="text-md text-ink-600">{stage.progress.phase}</span>
								<span class="font-mono text-xs text-muted-foreground tabular-nums">
									{stage.progress.meta}
								</span>
							</div>
							<Progress value={stage.progress.percent} aria-label={stage.progress.phase} />
						</div>

						<ol
							class="flex max-h-45 flex-col gap-1 overflow-y-auto rounded-lg border border-line-strong bg-paper-0 px-3 py-2.5"
						>
							{#each stage.log as entry (entry.at + entry.message)}
								<li class="flex gap-2.5 font-mono text-xs leading-normal">
									<span class="flex-none text-muted-foreground tabular-nums">{entry.at}</span>
									<span
										class="min-w-0 flex-1 {entry.tone === 'warning'
											? 'text-warning'
											: entry.tone === 'active'
												? 'text-ink-900'
												: 'text-ink-600'}"
									>
										{entry.message}
									</span>
								</li>
							{/each}
						</ol>

						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							This keeps running if you close the tab. We'll email you at
							<span class="font-mono text-ink-600">{stage.notifyEmail}</span>
							when it finishes.
						</p>
					</div>
				{/if}

				{#if stage.kind === "running" && stage.detached}
					<div class="flex flex-wrap items-start gap-5">
						<div class="flex min-w-65 flex-[1_1_300px] flex-col gap-2.5" aria-live="polite">
							<Progress value={stage.progress.percent} aria-label={stage.progress.phase} />
							<span class="font-mono text-xs text-muted-foreground">
								{stage.progress.phase} · {stage.progress.meta}
							</span>
						</div>
						<ul class="flex min-w-65 flex-[1_1_300px] flex-col gap-2">
							{#each [{ icon: Check, label: "Closing this tab won't stop it." }, { icon: Inbox, label: `We'll email ${stage.notifyEmail} when it finishes.` }, { icon: CircleAlert, label: "Imported issues appear as they're created, not all at the end." }] as note (note.label)}
								{@const NoteIcon = note.icon}
								<li class="flex items-start gap-2">
									<NoteIcon
										class="mt-px size-icon-row shrink-0 text-muted-foreground"
										aria-hidden="true"
									/>
									<span class="flex-1 text-md leading-normal text-ink-600">{note.label}</span>
								</li>
							{/each}
						</ul>
					</div>
				{/if}

				{#if stage.kind === "finished"}
					<div class="flex flex-col gap-4.5">
						<CountGrid counts={stage.outcome.counts} />

						{#if stage.outcome.kind === "with_skips"}
							{@const skipped = stage.outcome.skipped}
							<div class="flex flex-col gap-2">
								<div class="flex items-center justify-between gap-3">
									<Eyebrow class="text-ink-600">What was skipped</Eyebrow>
									<Button variant="ghost" size="sm">Download report</Button>
								</div>
								<Table.Root class="min-w-[420px]">
									<Table.Body>
										{#each skipped as row (row.kind)}
											<Table.Row>
												<Table.Cell
													class="w-27.5 font-mono text-sm text-muted-foreground"
												>
													{row.kind}
												</Table.Cell>
												<Table.Cell class="whitespace-normal text-ink-600">{row.reason}</Table.Cell>
												<Table.Cell
													data-align="right"
													class="w-20 font-mono text-sm text-muted-foreground tabular-nums"
												>
													{row.count}
												</Table.Cell>
											</Table.Row>
										{/each}
									</Table.Body>
								</Table.Root>
							</div>
						{/if}

						<p class="text-sm leading-normal text-muted-foreground text-pretty">{resultNote}</p>
					</div>
				{/if}
			</div>

			<div
				class="flex flex-wrap items-center gap-2 border-t border-line-subtle bg-card px-5 py-3.5 sm:px-5.5"
			>
				{#if footnote}
					<span class="flex-[1_1_200px] font-mono text-xs {footnote.tone}">{footnote.label}</span>
				{/if}
				<div class="flex-1"></div>
				{#if showBack}
					<Button variant="ghost">
						<ArrowLeft aria-hidden="true" />
						Back
					</Button>
				{/if}
				{#if secondary}
					<Button variant="secondary">{secondary}</Button>
				{/if}
				{#if cta.href}
					<Button href={cta.href}>{cta.label}</Button>
				{:else}
					<Button type="submit" form={cta.form} disabled={busy}>{cta.label}</Button>
				{/if}
			</div>
		</div>
	</div>
</div>

{#if stage.kind === "running" && stage.detached}
	<aside
		class="notch fixed right-6 bottom-[calc(--spacing(6)+env(safe-area-inset-bottom))] z-70 w-70 max-w-[calc(100vw-3rem)]"
		aria-label="Import progress"
	>
		<div class="flex flex-col gap-2 px-3.5 py-3">
			<div class="flex items-baseline justify-between gap-2">
				<span class="text-md font-medium text-ink-900">Importing from {sourceName}</span>
				<span class="font-mono text-xs text-muted-foreground tabular-nums">
					{stage.progress.percent}%
				</span>
			</div>
			<Progress value={stage.progress.percent} aria-label="Import progress" />
			<span class="font-mono text-xs text-muted-foreground">{stage.progress.meta}</span>
		</div>
	</aside>
{/if}
