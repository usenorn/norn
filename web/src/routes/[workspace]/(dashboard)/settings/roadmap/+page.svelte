<script lang="ts">
	import { tick, untrack } from "svelte";
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import MapIcon from "@lucide/svelte/icons/map";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { deploymentPreview } from "$lib/auth/preview";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Retry from "$lib/components/norn/retry.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import SaveBar from "$lib/settings/save-bar.svelte";
	import SettingRow from "$lib/settings/setting-row.svelte";
	import SettingsPage from "$lib/settings/settings-page.svelte";
	import { showToast } from "$lib/toast/toasts";
	import { saveBarOf } from "$lib/workspace/settings";
	import RoadmapEntryEditor from "$lib/roadmap/roadmap-entry-editor.svelte";
	import RoadmapEntryList from "$lib/roadmap/roadmap-entry-list.svelte";
	import RoadmapPresetChooser from "$lib/roadmap/roadmap-preset-chooser.svelte";
	import RoadmapPublicationHeader from "$lib/roadmap/roadmap-publication-header.svelte";
	import RoadmapRemoveEntryDialog from "$lib/roadmap/roadmap-remove-entry-dialog.svelte";
	import RoadmapReviewDialog from "$lib/roadmap/roadmap-review-dialog.svelte";
	import RoadmapSourcePicker from "$lib/roadmap/roadmap-source-picker.svelte";
	import RoadmapUnpublishDialog from "$lib/roadmap/roadmap-unpublish-dialog.svelte";
	import {
		answerOf,
		chosenPresets,
		failureMessage,
		newEntryContent,
		presetConflicts,
		presetKinds,
		presetOf,
		presetVisibility,
		publicationLabels,
		publicationOf,
		roadmapSettingsFields,
		selectedSources,
		settingsDraftAfter,
		settingsInputOf,
		visibilityLabels,
		type ChosenPreset,
		type EntryEditor,
		type EntrySource,
		type ReviewDialog,
		type Roadmap,
		type RoadmapActivity,
		type RoadmapAnswer,
		type RoadmapEntry,
		type RoadmapFailure,
		type RoadmapPreset,
		type RoadmapSettingsView,
		type VisibilityField,
	} from "$lib/roadmap/admin";
	import { roadmapSettingsSchema, type RoadmapSettingsInput } from "$lib/roadmap/roadmap-settings-schema";
	import { roadmapSettingsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "roadmap-settings-form";

	const visibilityHints: Record<VisibilityField, string> = {
		issueLists: "List the issues you selected under each project.",
		projectDescriptions: "Show projects' public descriptions. Each entry can still hide its own.",
		issueDescriptions: "Show issues' public descriptions. Each entry can still hide its own.",
		status: "Show each entry's public status.",
		timeline: "Show the dates you set on each entry.",
		progress: "Show progress on projects that have it turned on.",
	};

	const visibilityOrder: VisibilityField[] = [
		"issueLists",
		"projectDescriptions",
		"issueDescriptions",
		"status",
		"timeline",
		"progress",
	];

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? roadmapSettingsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let liveActivity = $state.raw<RoadmapActivity>({ kind: "idle" });
	let liveFailure = $state.raw<RoadmapFailure | null>(null);
	let chosen = $state<ChosenPreset | undefined>();
	const reviewLinked = $derived(page.url.searchParams.get("review") === "1");

	let editor = $derived<EntryEditor>(preview?.editor ?? { kind: "closed" });
	let review = $derived<ReviewDialog>(
		preview?.review ?? (reviewLinked && !preview ? { kind: "loading" } : { kind: "closed" })
	);
	let removalId = $derived(preview?.confirm?.kind === "remove" ? preview.confirm.entryId : undefined);
	let confirmingUnpublish = $derived(preview?.confirm?.kind === "unpublish");
	let titleInput = $state<HTMLElement | null>(null);
	let baseline: RoadmapSettingsInput | undefined;

	const view = $derived<RoadmapSettingsView>(preview?.view ?? data.view);
	const roadmap = $derived<Roadmap | undefined>(view.kind === "ready" ? view.roadmap : undefined);
	const projects = $derived(view.kind === "ready" ? view.projects : []);
	const activity = $derived<RoadmapActivity>(preview?.activity ?? liveActivity);
	const failure = $derived<RoadmapFailure | null>(preview?.failure ?? liveFailure);
	const selfHosted = $derived(deploymentPreview(page.url)?.selfHosted ?? data.selfHosted);
	const timezone = $derived(page.data.session?.account?.timezone ?? "UTC");
	const workspace = $derived(data.workspace);

	const publication = $derived(roadmap ? publicationOf(roadmap) : undefined);
	const preset = $derived<RoadmapPreset>(roadmap ? presetOf(roadmap) : "unstarted");
	const intended = $derived<ChosenPreset | undefined>(
		chosen ?? (isChosenPreset(preset) ? preset : roadmap?.visibility.issueLists ? "projects_with_issues" : undefined)
	);
	const conflicts = $derived(roadmap && intended ? presetConflicts(roadmap, intended) : []);
	const kinds = $derived(presetKinds(intended ?? preset));
	const selected = $derived(roadmap ? selectedSources(roadmap) : new Set<string>());
	const removal = $derived(roadmap?.entries.find((entry) => entry.id === removalId));
	const busy = $derived(activity.kind !== "idle");

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: formId,
		validators: zod4Client(roadmapSettingsSchema),
		resetForm: false,
		invalidateAll: false,
		applyAction: "never",
		onUpdated: async ({ form: updated }) => {
			const outcome = updated.message;

			if (outcome?.kind === "saved") {
				baseline = updated.data;
				reset({ data: updated.data });
				await invalidate(keys.roadmap(workspace.id));
				showToast("Saved");

				return;
			}

			if (outcome?.kind === "failed" && outcome.failure.kind === "stale") await invalidate(keys.roadmap(workspace.id));
		},
	});
	const { form: formData, enhance, submitting, message, tainted, isTainted, reset } = form;

	const saving = $derived($submitting || preview?.saving === true);
	const settingsFailure = $derived(
		preview?.outcome?.kind === "failed"
			? preview.outcome.failure
			: $message?.kind === "failed"
				? $message.failure
				: undefined
	);
	const bar = $derived(saveBarOf(isTainted($tainted) || preview?.saving === true, saving, false));

	$effect(() => {
		if (!roadmap) return;

		const latest = settingsInputOf(roadmap);

		untrack(() => {
			const touched = roadmapSettingsFields.filter((field) => isTainted(field));

			if (!baseline || touched.length === 0) {
				formData.set(latest, { taint: false });
				baseline = latest;

				return;
			}

			const merged = settingsDraftAfter(baseline, latest, $formData, touched);

			if (!merged) return;

			formData.set(merged, { taint: false });
			baseline = latest;
		});
	});

	function isChosenPreset(value: RoadmapPreset): value is ChosenPreset {
		return chosenPresets.some((option) => option === value);
	}

	async function run(next: RoadmapActivity, call: () => Promise<RoadmapAnswer>): Promise<boolean> {
		liveActivity = next;
		liveFailure = null;

		const answer = await call();

		await invalidate(keys.roadmap(workspace.id));
		liveActivity = { kind: "idle" };

		if (answer.kind === "refused") {
			liveFailure = answer.failure;

			return false;
		}

		return true;
	}

	async function choosePreset(next: ChosenPreset) {
		if (!roadmap) return;

		chosen = next;

		const visibility = presetVisibility(roadmap.visibility, next);

		if (visibility.issueLists === roadmap.visibility.issueLists) return;

		const { version, title, description } = roadmap;

		await run({ kind: "choosing_preset", preset: next }, () =>
			answerOf(
				api.PATCH("/workspaces/{workspaceId}/roadmap", {
					params: { path: { workspaceId: workspace.id } },
					body: { expectedVersion: version, title, description, visibility },
				})
			)
		);
	}

	async function refresh(entry: RoadmapEntry) {
		if (!roadmap) return;

		const expectedVersion = roadmap.version;

		const refreshed = await run({ kind: "refreshing", entryId: entry.id }, () =>
			answerOf(
				api.POST("/workspaces/{workspaceId}/roadmap/entries/{entryId}/refresh", {
					params: { path: { workspaceId: workspace.id, entryId: entry.id } },
					body: { expectedVersion },
				})
			)
		);

		if (refreshed) showToast("Status and date refreshed in the draft");
	}

	async function remove(entry: RoadmapEntry) {
		if (!roadmap) return;

		const expectedVersion = roadmap.version;

		const removed = await run({ kind: "removing", entryId: entry.id }, () =>
			answerOf(
				api.DELETE("/workspaces/{workspaceId}/roadmap/entries/{entryId}", {
					params: { path: { workspaceId: workspace.id, entryId: entry.id }, query: { expectedVersion } },
				})
			)
		);

		removalId = undefined;

		if (removed) showToast(entry.published ? "Removed from the roadmap and the public page" : "Removed from the roadmap");
	}

	async function unpublish() {
		const unpublished = await run({ kind: "unpublishing" }, () =>
			answerOf(api.POST("/workspaces/{workspaceId}/roadmap/unpublish", { params: { path: { workspaceId: workspace.id } } }))
		);

		confirmingUnpublish = false;

		if (unpublished) showToast("The public roadmap is down");
	}

	function edit(entry: RoadmapEntry) {
		editor = { kind: "open", target: { kind: "edit", entry }, standing: { kind: "editing" } };
	}

	function add(source: EntrySource) {
		if (!roadmap) return;

		editor = {
			kind: "open",
			target: { kind: "add", source, content: newEntryContent(roadmap.entries) },
			standing: { kind: "editing" },
		};
	}

	function editFromReview(entryId: string) {
		const entry = roadmap?.entries.find((candidate) => candidate.id === entryId);

		review = { kind: "closed" };

		if (entry) edit(entry);
	}

	async function editTitleFromReview() {
		review = { kind: "closed" };

		await tick();
		titleInput?.focus();
	}

	async function saveMineAnyway() {
		if (!roadmap) return;

		baseline = settingsInputOf(roadmap);
		$formData.expectedVersion = roadmap.version;

		await tick();
		form.submit();
	}

	function discardMine() {
		if (!roadmap) return;

		baseline = settingsInputOf(roadmap);
		reset({ data: baseline });
		message.set(undefined);
	}
</script>

<svelte:head><title>Public roadmap · {workspace.name} · Norn</title></svelte:head>

<SettingsPage
	title="Public roadmap"
	description="Choose what the public sees, write the public copy, preview it and publish."
	Icon={MapIcon}
	meta={publication ? publicationLabels[publication.kind] : undefined}
>
	{#snippet toolbar()}
		{#if roadmap}
			<SaveBar {bar} {formId} ondiscard={discardMine} />
		{/if}
	{/snippet}

	{#if view.kind === "loading"}
		<div class="flex flex-col gap-3" aria-busy="true" aria-label="Loading the public roadmap">
			<Skeleton class="h-28 w-full" />
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-48 w-full" />
		</div>
	{:else if view.kind === "forbidden"}
		<Alert.Root variant="muted">
			<div class="col-span-full flex flex-col gap-0.5">
				<Eyebrow>No access · admin only</Eyebrow>
				<Alert.Title>Only administrators manage the public roadmap</Alert.Title>
				<Alert.Description>
					What {workspace.name} shares publicly is decided by its administrators. Ask one of them to change it.
				</Alert.Description>
			</div>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>Something went wrong</Alert.Title>
			<Alert.Description>We could not read the roadmap. Wait a moment and try again.</Alert.Description>
			<Alert.Action><Retry /></Alert.Action>
		</Alert.Root>
	{:else if roadmap && publication}
		<RoadmapPublicationHeader
			{publication}
			{activity}
			{selfHosted}
			members={data.members}
			{timezone}
			workspace={workspace.slug}
			onreview={() => (review = { kind: "loading" })}
			onunpublish={() => (confirmingUnpublish = true)}
		/>

		<div aria-live="polite" class="flex flex-col gap-3 empty:hidden">
			{#if failure}
				<Alert.Root variant={failure.kind === "stale" ? "warning" : "destructive"}>
					{#if failure.kind === "stale"}
						<TriangleAlert aria-hidden="true" />
					{:else}
						<CircleAlert aria-hidden="true" />
					{/if}
					<Alert.Title>{failure.kind === "stale" ? "The roadmap changed" : "That did not work"}</Alert.Title>
					<Alert.Description>
						{failure.kind === "stale"
							? "Someone else changed the roadmap at the same moment. It now shows their version; try again if you still want the change."
							: failureMessage(failure)}
					</Alert.Description>
				</Alert.Root>
			{/if}
		</div>

		<RoadmapPresetChooser
			{preset}
			{intended}
			{conflicts}
			{activity}
			onchoose={choosePreset}
			onremove={(entry) => (removalId = entry.id)}
		/>

		<section class="flex flex-col gap-3" aria-labelledby="roadmap-entries">
			<div class="flex flex-wrap items-end justify-between gap-3">
				<div class="flex min-w-0 flex-col gap-0.5">
					<h2 id="roadmap-entries" class="text-md font-medium tracking-snug text-ink-900">
						Entries
						<span class="font-mono text-xs text-muted-foreground">{roadmap.entries.length}</span>
					</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Each entry has its own public copy. Internal names, descriptions and comments are never published.
					</p>
				</div>
				<RoadmapSourcePicker
					{kinds}
					{projects}
					{selected}
					workspaceId={workspace.id}
					workspace={workspace.slug}
					disabled={busy}
					opened={preview?.picker?.opened}
					outcome={preview?.picker?.outcome}
					onpick={add}
				/>
			</div>

			<RoadmapEntryList
				entries={roadmap.entries}
				visibility={roadmap.visibility}
				{activity}
				onedit={edit}
				onrefresh={refresh}
				onremove={(entry) => (removalId = entry.id)}
			/>
		</section>

		<form
			id={formId}
			method="POST"
			action="?/settings"
			use:enhance
			class="flex flex-col gap-4"
			aria-labelledby="roadmap-page"
		>
			<input type="hidden" name="workspaceId" value={workspace.id} />
			<input type="hidden" name="expectedVersion" value={$formData.expectedVersion} />

			<div class="flex flex-col gap-0.5">
				<h2 id="roadmap-page" class="text-md font-medium tracking-snug text-ink-900">Roadmap page</h2>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					The heading readers see, and what the whole roadmap is allowed to show. An entry can hide more, never
					less.
				</p>
			</div>

			{#if settingsFailure}
				<Alert.Root variant={settingsFailure.kind === "stale" ? "warning" : "destructive"}>
					{#if settingsFailure.kind === "stale"}
						<TriangleAlert aria-hidden="true" />
					{:else}
						<CircleAlert aria-hidden="true" />
					{/if}
					<Alert.Title>The roadmap page was not saved</Alert.Title>
					<Alert.Description>
						<span class="block">{failureMessage(settingsFailure)}</span>
						{#if settingsFailure.kind === "stale"}
							<span class="mt-2 flex flex-wrap gap-2">
								<Button size="sm" variant="secondary" disabled={saving} onclick={saveMineAnyway}>Save mine anyway</Button>
								<Button size="sm" variant="ghost" disabled={saving} onclick={discardMine}>Discard mine</Button>
							</span>
						{/if}
					</Alert.Description>
				</Alert.Root>
			{/if}

			<Form.Field {form} name="title">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Public title</Form.Label>
						<Input
							{...props}
							bind:ref={titleInput}
							bind:value={$formData.title}
							disabled={saving}
							autocomplete="off"
							placeholder="Northwind roadmap"
						/>
					{/snippet}
				</Form.Control>
				<Form.Description>Required before the roadmap can be published.</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="description">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Public description</Form.Label>
						<Textarea {...props} rows={4} bind:value={$formData.description} disabled={saving} />
					{/snippet}
				</Form.Control>
				<Form.Description>Plain text shown under the title. Optional.</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<div class="flex flex-col divide-y divide-line-default rounded-md border border-line-default">
				{#each visibilityOrder as field (field)}
					<SettingRow {form} name={field} label={visibilityLabels[field]} hint={visibilityHints[field]}>
						{#snippet control(props)}
							<Switch {...props} bind:checked={$formData[field]} disabled={saving} class="sm:justify-self-end" />
						{/snippet}
					</SettingRow>
				{/each}
			</div>

			<div class="flex flex-wrap gap-2">
				<Button type="submit" disabled={saving}>{saving ? "Saving" : "Save roadmap page"}</Button>
			</div>
		</form>

		{#if editor.kind === "open"}
			{#key editor.target}
				<RoadmapEntryEditor
					target={editor.target}
					standing={editor.standing}
					{roadmap}
					workspaceId={workspace.id}
					source={preview?.source}
					onstanding={(standing) => {
						if (editor.kind === "open") editor = { ...editor, standing };
					}}
					onclose={() => (editor = { kind: "closed" })}
					onsaved={() => {
						const added = editor.kind === "open" && editor.target.kind === "add";

						editor = { kind: "closed" };
						showToast(added ? "Added to the draft" : "Saved in the draft");
					}}
				/>
			{/key}
		{/if}

		{#if review.kind !== "closed"}
			<RoadmapReviewDialog
				dialog={review}
				entries={roadmap.entries}
				published={roadmap.publication !== undefined}
				workspaceId={workspace.id}
				workspace={workspace.slug}
				live={!preview}
				onchange={(next) => (review = next)}
				onpublished={() => {
					review = { kind: "closed" };
					showToast("Published");
				}}
				oneditentry={editFromReview}
				oneditroadmap={editTitleFromReview}
			/>
		{/if}

		<RoadmapUnpublishDialog
			open={confirmingUnpublish}
			unpublishing={activity.kind === "unpublishing"}
			onconfirm={unpublish}
			oncancel={() => (confirmingUnpublish = false)}
		/>

		<RoadmapRemoveEntryDialog
			entry={removal}
			entries={roadmap.entries}
			removing={activity.kind === "removing"}
			onconfirm={remove}
			oncancel={() => (removalId = undefined)}
		/>
	{/if}
</SettingsPage>
