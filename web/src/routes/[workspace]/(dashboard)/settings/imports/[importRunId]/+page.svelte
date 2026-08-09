<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { keys } from "$lib/api/keys";
	import { onDateAndTime } from "$lib/time";
	import CsvFileForm from "$lib/imports/csv-file-form.svelte";
	import CsvShapeForm from "$lib/imports/csv-shape-form.svelte";
	import LinearScopeForm from "$lib/imports/linear-scope-form.svelte";
	import MappingForm from "$lib/imports/mapping-form.svelte";
	import PreviewForm from "$lib/imports/preview-form.svelte";
	import ReportPanel from "$lib/imports/report-panel.svelte";
	import RunProgress from "$lib/imports/run-progress.svelte";
	import SourceKeyForm from "$lib/imports/source-key-form.svelte";
	import { readCatalogue, readStalePreview } from "$lib/imports/steps";
	import {
		counted,
		csvSettings,
		failureMessage,
		failureTitle,
		importsPath,
		pollIntervalMs,
		sourceName,
		statusLabel,
		statusTone,
		working,
		type ImportFailure,
		type ImportMappingPlan,
		type ImportRunView,
	} from "$lib/imports/imports";
	import { importRunPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? importRunPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let awaitingFrom = $state<string | null>(null);
	let loaded = $state.raw<ImportRunView | null>(null);
	let localFailure = $state.raw<ImportFailure | null>(null);
	let replacingKey = $state(false);
	let retrying = $state(false);

	const workspace = $derived(page.data.workspace);
	const view = $derived<ImportRunView>(loaded ?? preview?.view ?? data.view);
	const failure = $derived(preview?.failure ?? localFailure);
	const run = $derived("run" in view ? view.run : null);
	const title = $derived(
		run ? run.sourceLabel || sourceName(run.sourceKind) : "Import"
	);

	async function advanced() {
		loaded = null;
		localFailure = null;
		awaitingFrom = run?.status ?? null;

		await invalidate(keys.page(page.route.id));
	}

	async function stale() {
		if (!run) return;

		const result = await readStalePreview(workspace.id, run);

		if (result.ok) {
			loaded = result.view;
			localFailure = { kind: "preview_stale" };
		} else {
			localFailure = result.failure;
		}
	}

	function show(next: ImportRunView) {
		loaded = next;
		localFailure = null;
		replacingKey = false;
	}

	function fail(next: ImportFailure | null) {
		localFailure = next;
	}

	function replan(plan: ImportMappingPlan) {
		if (view.kind !== "mapping") return;

		loaded = { ...view, plan };
	}

	async function retry() {
		if (!run) return;

		retrying = true;
		localFailure = null;

		try {
			const result = await readCatalogue(workspace.id, run);

			if (result.ok) {
				loaded = result.view;
			} else {
				localFailure = result.failure;
			}
		} finally {
			retrying = false;
		}
	}

	$effect(() => {
		if (!working(view.kind) && awaitingFrom === null) return;

		const timer = setInterval(() => {
			void invalidate(keys.page(page.route.id));
		}, pollIntervalMs);

		return () => clearInterval(timer);
	});

	// A phase is asked for over HTTP and carried out by a worker, so the run has not moved yet
	// when the request comes back. Reading it once at that moment sees the state it was already
	// in, and a screen that only polls while the run is working would stop looking before it
	// ever started: the phase runs, finishes, and the operator watches a button that appears to
	// have done nothing. Waiting on the status changing rather than on it becoming a working one
	// also covers a small file, which is read and settled between two polls.
	$effect(() => {
		if (awaitingFrom !== null && run && run.status !== awaitingFrom) awaitingFrom = null;
	});
</script>

<svelte:head><title>{title} · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-3 border-b border-line-subtle px-4">
		<a
			href={importsPath(workspace.slug)}
			class="flex items-center gap-1.5 text-sm text-muted-foreground motion-control hover:text-ink-900"
		>
			<ArrowLeft class="size-4" aria-hidden="true" />
			<span>Imports</span>
		</a>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-200 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant={failure.kind === "would_triage" ? "warning" : "destructive"}>
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>{failureTitle(failure)}</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if view.kind === "not_found"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That import is not here</Alert.Title>
					<Alert.Description>
						It belongs to another workspace, or it was never started.
					</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not run imports here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load this import</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading this import…</p>
			{:else if run}
				<section class="flex flex-col gap-2">
					<div class="flex flex-wrap items-center gap-2">
						<h1 class="text-lg font-medium tracking-snug text-ink-900">{title}</h1>
						<Tag name={statusLabel(run.status)} color={statusTone(run.status)} />
					</div>
					<p class="text-sm text-muted-foreground">
						{sourceName(run.sourceKind)} · started {onDateAndTime(
							run.createdAt,
							workspace.timezone
						)}
						{#if run.finishedAt}
							· finished {onDateAndTime(run.finishedAt, workspace.timezone)}
						{/if}
					</p>
				</section>

				<RunProgress {run} busy={working(view.kind)} />

				{#if view.kind === "connect"}
					<section class="flex flex-col gap-5">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								Point this run at the backlog
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Nothing is read until you say which part of the source to read, and nothing is
								created until you have seen what would be created.
							</p>
						</div>

						{#if run.sourceKind === "csv"}
							<CsvFileForm
								workspaceId={workspace.id}
								{run}
								onview={show}
								onfailure={fail}
							/>
						{:else}
							<SourceKeyForm
								workspaceId={workspace.id}
								{run}
								settings={run.settings ?? {}}
								submitLabel="Save the key and read what it can see"
								pendingLabel="Asking the source…"
								onview={show}
								onfailure={fail}
							/>
						{/if}
					</section>
				{:else if view.kind === "catalogue"}
					{#if run.sourceKind === "csv"}
						{@const file = csvSettings(run)}
						<Alert.Root variant="muted">
							<CircleAlert aria-hidden="true" />
							<Alert.Title>Reading {file.fileName || "the uploaded file"}</Alert.Title>
							<Alert.Description>
								Upload a different file by starting another import; this run keeps the file it was
								given.
							</Alert.Description>
						</Alert.Root>

						<CsvShapeForm
							workspaceId={workspace.id}
							{run}
							catalogue={view.catalogue}
							onview={show}
							onadvanced={advanced}
							onfailure={fail}
						/>
					{:else}
						{#if replacingKey}
							<SourceKeyForm
								workspaceId={workspace.id}
								{run}
								settings={run.settings ?? {}}
								askPolicy={false}
								submitLabel="Use this key instead"
								pendingLabel="Asking the source…"
								oncancel={() => (replacingKey = false)}
								onview={show}
								onfailure={fail}
							/>
						{:else}
							<Alert.Root variant="muted">
								<CircleAlert aria-hidden="true" />
								<Alert.Title>A key is stored for this run</Alert.Title>
								<Alert.Description>
									It is held encrypted and is never shown again, here or anywhere else. Replace it if
									you gave the wrong one.
								</Alert.Description>
								<Alert.Action>
									<Button variant="secondary" size="sm" onclick={() => (replacingKey = true)}>
										Replace the key
									</Button>
								</Alert.Action>
							</Alert.Root>

							<LinearScopeForm
								workspaceId={workspace.id}
								{run}
								catalogue={view.catalogue}
								onadvanced={advanced}
								onfailure={fail}
							/>
						{/if}
					{/if}
				{:else if view.kind === "staging"}
					<section class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Reading the source</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn is draining the source into a copy of its own. Nothing in {workspace.name} has
							changed and nothing will until you approve what this would do. You can leave this page;
							the run carries on without it.
						</p>
					</section>
				{:else if view.kind === "rate_limited"}
					<section class="flex flex-col gap-4 rounded-lg border border-warning/40 p-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								The source is throttling this instance
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								{#if view.resumesAt}
									It has asked to be left alone until {onDateAndTime(
										view.resumesAt,
										workspace.timezone
									)}. Nothing is lost — what was already read stays read, and the run picks up where
									it stopped.
								{:else}
									It has asked to be left alone for a while. Nothing is lost — what was already read
									stays read, and the run picks up where it stopped.
								{/if}
							</p>
						</div>

						<div>
							<Button variant="secondary" size="sm" disabled={retrying} onclick={retry}>
								{retrying ? "Asking again…" : "Ask the source again"}
							</Button>
						</div>
					</section>
				{:else if view.kind === "source_refused"}
					<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								The source would not hand that over
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								{view.reason ??
									"The source answered, but not with what this run asked it for. That is usually a key without enough access, or a workspace it cannot see."}
							</p>
						</div>

						<div class="flex flex-wrap gap-2">
							<Button variant="secondary" size="sm" disabled={retrying} onclick={retry}>
								{retrying ? "Asking again…" : "Ask the source again"}
							</Button>
							{#if run.sourceKind !== "csv"}
								<Button variant="ghost" size="sm" onclick={() => show({ kind: "connect", run })}>
									Use a different key
								</Button>
							{/if}
						</div>
					</section>
				{:else if view.kind === "encryption_unavailable"}
					<section class="flex flex-col gap-3 rounded-lg border border-line-default bg-paper-0 p-5">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								This instance cannot hold a source key
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								A key for the source you are importing from is stored encrypted, and this instance
								has no encryption key to seal it with. Until it has one, no import that needs a key
								can be configured or read.
							</p>
						</div>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							An operator sets <span class="font-mono text-ink-900">NORN_SECURITY_ENCRYPTION_KEY</span>
							to 32 base64-encoded random bytes and restarts the instance. This run keeps its place and
							carries on from here afterwards.
						</p>
					</section>
				{:else if view.kind === "staged"}
					<section class="flex flex-col gap-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">The source has been read</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								{counted(run.staged, "record is", "records are")} held in this run's own copy, and
								{workspace.name} is untouched. Next, say what each thing the source named becomes
								here.
								{#if view.plan.mappings.length > 0}
									{counted(view.plan.mappings.length, "concept", "concepts")} came back;
									{view.plan.complete
										? "all of them are decided."
										: `${view.plan.mappings.filter((mapping) => !mapping.decision).length} still have no answer.`}
								{/if}
							</p>
						</div>

						<div>
							<Button onclick={() => show({ ...view, kind: "mapping" })}>
								Decide what becomes what
							</Button>
						</div>
					</section>
				{:else if view.kind === "mapping"}
					<MappingForm
						workspaceId={workspace.id}
						{run}
						plan={view.plan}
						targets={view.targets}
						onplan={replan}
						onadvanced={advanced}
						onfailure={fail}
					/>
				{:else if view.kind === "preview"}
					<PreviewForm
						workspaceId={workspace.id}
						{run}
						preview={view.preview}
						onadvanced={advanced}
						onstale={stale}
						onfailure={fail}
					/>
				{:else if view.kind === "triage_ack"}
					<PreviewForm
						workspaceId={workspace.id}
						{run}
						preview={view.preview}
						teams={view.teams}
						onadvanced={advanced}
						onstale={stale}
						onfailure={fail}
					/>
				{:else if view.kind === "preview_stale"}
					<PreviewForm
						workspaceId={workspace.id}
						{run}
						preview={view.preview}
						teams={run.acknowledgeTriage ? [] : view.preview.triageTeams}
						stale
						onadvanced={advanced}
						onstale={stale}
						onfailure={fail}
					/>
				{:else if view.kind === "executing"}
					<section class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Importing</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn is applying exactly what you approved, recording each thing it creates so the run
							can be taken back afterwards. You can leave this page; it carries on without it.
						</p>
					</section>
				{:else if view.kind === "reverting"}
					<section class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Taking the import back</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn is removing what this run created, in the reverse of the order it created it.
							Anything a person has touched since is kept.
						</p>
					</section>
				{:else if view.kind === "imported"}
					<Alert.Root variant="success">
						<CircleCheck aria-hidden="true" />
						<Alert.Title>The import finished</Alert.Title>
						<Alert.Description>
							{counted(run.processed, "record was handled", "records were handled")}. Everything it
							created is listed below and can be taken back for as long as this run is kept.
						</Alert.Description>
					</Alert.Root>

					<ReportPanel
						workspaceId={workspace.id}
						{run}
						report={view.report}
						timezone={workspace.timezone}
						onadvanced={advanced}
						onfailure={fail}
					/>
				{:else if view.kind === "reverted"}
					<Alert.Root variant="muted">
						<CircleAlert aria-hidden="true" />
						<Alert.Title>This import has been taken back</Alert.Title>
						<Alert.Description>
							What it created has been removed, apart from anything a person had touched since. The
							record of the run is kept so you can see what happened.
						</Alert.Description>
					</Alert.Root>

					<ReportPanel
						workspaceId={workspace.id}
						{run}
						report={view.report}
						timezone={workspace.timezone}
						onadvanced={advanced}
						onfailure={fail}
					/>
				{:else if view.kind === "failed"}
					<Alert.Root variant="destructive">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>This import stopped partway</Alert.Title>
						<Alert.Description>
							{run.phaseError ?? "The run stopped before it finished."}
							{counted(run.processed, "record was handled", "records were handled")} before it
							stopped. What it had already created is still here and is listed below; taking the run
							back removes exactly that.
						</Alert.Description>
					</Alert.Root>

					<ReportPanel
						workspaceId={workspace.id}
						{run}
						report={view.report}
						timezone={workspace.timezone}
						onadvanced={advanced}
						onfailure={fail}
					/>
				{/if}
			{/if}
		</div>
	</div>
</div>
