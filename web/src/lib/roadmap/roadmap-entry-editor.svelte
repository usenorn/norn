<script lang="ts">
	import { onMount } from "svelte";
	import { invalidate } from "$app/navigation";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "#lib/api/index.js";
	import { keys } from "#lib/api/keys.js";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Dialog from "#lib/components/ui/dialog/index.js";
	import * as Form from "#lib/components/ui/form/index.js";
	import * as RadioGroup from "#lib/components/ui/radio-group/index.js";
	import * as Select from "#lib/components/ui/select/index.js";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import RowGroup from "#lib/components/norn/row-group.svelte";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Skeleton } from "#lib/components/ui/skeleton/index.js";
	import { Switch } from "#lib/components/ui/switch/index.js";
	import { Textarea } from "#lib/components/ui/textarea/index.js";
	import SettingRow from "#lib/settings/setting-row.svelte";
	import { statusLabels, timelineLabel } from "./public-roadmap";
	import {
		answerOf,
		contentOfInput,
		effectiveFlag,
		entryContentLabels,
		entryContentOf,
		entryFields,
		entryInputOf,
		failureMessage,
		flagEffectLabels,
		issueSourceText,
		progressCoverageLine,
		projectSourceText,
		roadmapFieldMessage,
		staleStanding,
		visibilityLabels,
		type EntryEditorStanding,
		type EntryEditorTarget,
		type Roadmap,
		type RoadmapEntry,
		type SourceText,
	} from "./admin";
	import { roadmapEntrySchema, roadmapProgressModes, roadmapStatuses } from "./roadmap-entry-schema";

	let {
		target,
		standing,
		roadmap,
		workspaceId,
		source: shownSource,
		onstanding,
		onclose,
		onsaved,
	}: {
		target: EntryEditorTarget;
		standing: EntryEditorStanding;
		roadmap: Roadmap;
		workspaceId: string;
		source?: SourceText;
		onstanding: (standing: EntryEditorStanding) => void;
		onclose: () => void;
		onsaved: (roadmap: Roadmap) => void;
	} = $props();

	const formPaths = [
		"publicTitle",
		"publicDescription",
		"status",
		"timeline.startOn",
		"timeline.endOn",
		"progressMode",
		"progressPercent",
	] as const;

	type FormPath = (typeof formPaths)[number];

	const progressLabels: Record<(typeof roadmapProgressModes)[number], { title: string; hint: string }> = {
		hidden: { title: "Hidden", hint: "Readers see no progress for this project." },
		approved: { title: "A figure you approve", hint: "Readers see exactly the percentage you enter." },
		calculated: { title: "Calculated", hint: "Counted from the issues you selected under this project." },
	};

	// svelte-ignore state_referenced_locally
	const opened: RoadmapEntry | undefined = target.kind === "edit" ? target.entry : undefined;
	// svelte-ignore state_referenced_locally
	const kind = target.kind === "edit" ? target.entry.kind : target.source.kind;
	// svelte-ignore state_referenced_locally
	const position = target.kind === "edit" ? target.entry.position : (target.content.position ?? roadmap.entries.length);
	// svelte-ignore state_referenced_locally
	let version = $state(roadmap.version);
	let loaded = $state.raw<SourceText>({ kind: "loading" });
	let body = $state<HTMLFormElement | null>(null);

	const sourceText = $derived(shownSource ?? loaded);

	// svelte-ignore state_referenced_locally
	const initial = entryInputOf(target.kind === "edit" ? entryContentOf(target.entry) : target.content);

	const form = superForm(defaults(initial, zod4(roadmapEntrySchema)), {
		id: "roadmap-entry-form",
		SPA: true,
		dataType: "json",
		validators: zod4Client(roadmapEntrySchema),
		resetForm: false,
		applyAction: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			const sent = standing.kind === "stale" ? standing.version : version;
			const content = contentOfInput(entered.data, position);

			const answer = await answerOf(
				target.kind === "edit"
					? api.PATCH("/workspaces/{workspaceId}/roadmap/entries/{entryId}", {
							params: { path: { workspaceId, entryId: target.entry.id } },
							body: { expectedVersion: sent, content },
						})
					: api.POST("/workspaces/{workspaceId}/roadmap/entries", {
							params: { path: { workspaceId } },
							body: { expectedVersion: sent, kind, sourceId: target.source.id, content },
						})
			);

			if (answer.kind === "done") {
				await invalidate(keys.roadmap(workspaceId));
				onsaved(answer.roadmap);

				return;
			}

			const { failure } = answer;

			if (failure.kind === "stale") {
				const latest = await answerOf(api.GET("/workspaces/{workspaceId}/roadmap", { params: { path: { workspaceId } } }));

				await invalidate(keys.roadmap(workspaceId));
				onstanding(latest.kind === "done" ? staleStanding(opened, latest.roadmap) : { kind: "failed", failure });
				body?.scrollTo({ top: 0 });

				return;
			}

			if (failure.kind === "invalid") {
				const placed = failure.errors.filter((error) => isFormPath(error.field));

				for (const error of placed) {
					if (isFormPath(error.field)) setError(entered, error.field, roadmapFieldMessage(error));
				}

				if (placed.length === failure.errors.length) {
					onstanding({ kind: "editing" });

					return;
				}
			}

			onstanding({ kind: "failed", failure });
			body?.scrollTo({ top: 0 });
		},
	});

	const { form: formData, enhance, submitting } = form;

	const busy = $derived($submitting);
	const visibilityFields = $derived(entryFields(kind));
	const removedMeanwhile = $derived(standing.kind === "stale" && target.kind === "edit" && !standing.current);
	const datesEffect = $derived(effectiveFlag(roadmap.visibility, $formData.visibility, "timeline"));
	const disclosed = $derived(
		timelineLabel({
			startOn: $formData.timeline.startOn || undefined,
			endOn: $formData.timeline.endOn || undefined,
		})
	);
	const coverage = $derived(opened ? progressCoverageLine(opened.progressCoverage) : undefined);
	const heading = $derived(
		target.kind === "edit" ? `Edit ${target.entry.publicTitle}` : `Add ${target.source.label}`
	);

	function isFormPath(field: string): field is FormPath {
		return formPaths.some((path) => path === field);
	}

	function isProgressMode(value: string): value is (typeof roadmapProgressModes)[number] {
		return roadmapProgressModes.some((mode) => mode === value);
	}

	function isStatus(value: string): value is (typeof roadmapStatuses)[number] {
		return roadmapStatuses.some((status) => status === value);
	}

	function useTheirs(current: RoadmapEntry, latest: number) {
		form.reset({ data: entryInputOf(entryContentOf(current)) });
		version = latest;
		onstanding({ kind: "editing" });
	}

	async function readSource(): Promise<SourceText> {
		const projectId = target.kind === "edit" ? target.entry.projectId : kind === "project" ? target.source.id : undefined;
		const issueId = target.kind === "edit" ? target.entry.issueId : kind === "issue" ? target.source.id : undefined;

		if (target.kind === "edit" && !target.entry.reachable) return { kind: "unreadable" };

		try {
			if (kind === "project" && projectId) {
				const { data } = await api.GET("/workspaces/{workspaceId}/projects/{projectId}", {
					params: { path: { workspaceId, projectId } },
				});

				return data ? projectSourceText(data) : { kind: "unreadable" };
			}

			if (issueId) {
				const { data } = await api.GET("/workspaces/{workspaceId}/issues/{issueId}", {
					params: { path: { workspaceId, issueId } },
				});

				return data ? issueSourceText(data) : { kind: "unreadable" };
			}
		} catch {
			return { kind: "unreadable" };
		}

		return { kind: "unreadable" };
	}

	onMount(() => {
		if (shownSource) return;

		let current = true;

		readSource().then((text) => {
			if (current) loaded = text;
		});

		return () => {
			current = false;
		};
	});
</script>

<Dialog.Root
	open
	onOpenChange={(next) => {
		if (!next && !busy) onclose();
	}}
>
	<Dialog.Content
		variant="scrollable"
		class="sm:max-w-150"
		showCloseButton={!busy}
		onEscapeKeydown={(event) => busy && event.preventDefault()}
		onInteractOutside={(event) => busy && event.preventDefault()}
	>
		<form bind:this={body} method="POST" id="roadmap-entry-form" use:enhance class="flex min-h-0 flex-col gap-5 overflow-y-auto">
			<Dialog.Header>
				<Dialog.Title class="break-words">{heading}</Dialog.Title>
				<Dialog.Description>
					Readers see only the public copy below and the fields you turn on. Nothing changes publicly until you
					publish.
				</Dialog.Description>
			</Dialog.Header>

			<div aria-live="polite" class="flex flex-col gap-3 empty:hidden">
				{#if standing.kind === "stale"}
					<Alert.Root variant="warning">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>
							{removedMeanwhile ? "This entry was removed meanwhile" : "Someone else changed the roadmap"}
						</Alert.Title>
						<Alert.Description>
							{#if removedMeanwhile}
								<span class="block">It is no longer on the roadmap, so there is nothing to save it over.</span>
							{:else if standing.differences.length > 0}
								<span class="block">
									Changed on this entry meanwhile: {standing.differences
										.map((field) => entryContentLabels[field])
										.join(", ")}. Your typed values are still here.
								</span>
							{:else}
								<span class="block">
									Nothing on this entry changed, but the roadmap did. Your typed values are still here.
								</span>
							{/if}
							{#if standing.current && !removedMeanwhile}
								<span class="mt-2 flex flex-wrap gap-2">
									<Button
										size="sm"
										variant="secondary"
										disabled={busy}
										onclick={() => {
											if (standing.kind === "stale" && standing.current) useTheirs(standing.current, standing.version);
										}}
									>
										Discard mine and use theirs
									</Button>
								</span>
							{/if}
						</Alert.Description>
					</Alert.Root>
				{:else if standing.kind === "failed"}
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" />
						<Alert.Title>The entry was not saved</Alert.Title>
						<Alert.Description>{failureMessage(standing.failure)}</Alert.Description>
					</Alert.Root>
				{/if}
			</div>

			<section
				class="flex flex-col gap-2 rounded-md border border-dashed border-line-strong bg-paper-1 p-3"
				aria-labelledby="roadmap-entry-source"
			>
				<h3 id="roadmap-entry-source"><Eyebrow>Internal · not public</Eyebrow></h3>

				{#if sourceText.kind === "loading"}
					<div class="flex flex-col gap-2" aria-busy="true" aria-label="Reading the project or issue">
						<Skeleton class="h-4 w-48" />
						<Skeleton class="h-10 w-full" />
					</div>
				{:else if sourceText.kind === "unreadable"}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						The project or issue cannot be read right now, so there is nothing to copy. Write the public copy
						yourself.
					</p>
				{:else}
					<p class="text-sm break-words text-ink-900">
						{#if sourceText.reference}
							<span class="font-mono text-xs text-muted-foreground">{sourceText.reference}</span>
						{/if}
						{sourceText.label}
					</p>
					{#if sourceText.description}
						<p class="line-clamp-4 text-sm leading-normal break-words whitespace-pre-line text-muted-foreground">
							{sourceText.description}
						</p>
					{:else}
						<p class="text-sm text-muted-foreground">No description.</p>
					{/if}
					<div class="flex flex-wrap gap-2">
						<Button
							size="xs"
							variant="secondary"
							disabled={busy}
							onclick={() => {
								if (sourceText.kind === "ready") $formData.publicTitle = sourceText.label;
							}}
						>
							Copy into public title
						</Button>
						<Button
							size="xs"
							variant="secondary"
							disabled={busy || !sourceText.description}
							onclick={() => {
								if (sourceText.kind === "ready") $formData.publicDescription = sourceText.description;
							}}
						>
							Copy into public description
						</Button>
					</div>
				{/if}
			</section>

			<Form.Field {form} name="publicTitle">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Public title</Form.Label>
						<Input {...props} bind:value={$formData.publicTitle} disabled={busy} autocomplete="off" />
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="publicDescription">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Public description</Form.Label>
						<Textarea {...props} rows={5} bind:value={$formData.publicDescription} disabled={busy} />
					{/snippet}
				</Form.Control>
				<Form.Description>Plain text. Shown only while descriptions are turned on below.</Form.Description>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="status">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Public status</Form.Label>
						<Select.Root
							type="single"
							value={$formData.status ?? ""}
							disabled={busy}
							onValueChange={(value) => ($formData.status = isStatus(value) ? value : null)}
						>
							<Select.Trigger {...props} class="w-full">
								{$formData.status ? statusLabels[$formData.status] : "Take it from the project or issue"}
							</Select.Trigger>
							<Select.Content>
								{#if target.kind === "add"}
									<Select.Item value="" label="Take it from the project or issue">
										Take it from the project or issue
									</Select.Item>
								{/if}
								{#each roadmapStatuses as status (status)}
									<Select.Item value={status} label={statusLabels[status]}>{statusLabels[status]}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Fieldset {form} name="timeline" class="flex flex-col gap-3">
				<Form.Legend>Public dates</Form.Legend>
				<div class="flex flex-col gap-3 sm:flex-row">
					<Form.Field {form} name="timeline.startOn" class="flex-1">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Starts</Form.Label>
								<Input {...props} type="date" class="font-mono" bind:value={$formData.timeline.startOn} disabled={busy} />
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
					<Form.Field {form} name="timeline.endOn" class="flex-1">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Ends</Form.Label>
								<Input {...props} type="date" class="font-mono" bind:value={$formData.timeline.endOn} disabled={busy} />
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				</div>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					{#if datesEffect === "roadmap_off"}
						Dates are turned off for the whole roadmap, so readers see none of these.
					{:else if datesEffect === "entry_off"}
						Dates are turned off for this entry, so readers see none of these.
					{:else if disclosed}
						Readers will see: {disclosed}.
					{:else if target.kind === "add"}
						Left empty, the end date is taken from the project's or issue's target date, and readers see it.
					{:else}
						No dates set, so readers see none.
					{/if}
				</p>
			</Form.Fieldset>

			{#if kind === "project"}
				<Form.Fieldset {form} name="progressMode" class="flex flex-col gap-2">
					<Form.Legend>Public progress</Form.Legend>
					<RadioGroup.Root
						value={$formData.progressMode}
						onValueChange={(value) => {
							if (isProgressMode(value)) $formData.progressMode = value;
						}}
						disabled={busy}
						class="gap-2"
					>
						{#each roadmapProgressModes as mode (mode)}
							<div class="flex items-start gap-2">
								<RadioGroup.Item value={mode} id="roadmap-progress-{mode}" class="mt-0.5" />
								<label for="roadmap-progress-{mode}" class="flex min-w-0 flex-col gap-0.5">
									<span class="text-sm text-ink-900">{progressLabels[mode].title}</span>
									<span class="text-xs leading-normal text-muted-foreground text-pretty">{progressLabels[mode].hint}</span>
								</label>
							</div>
						{/each}
					</RadioGroup.Root>
					<Form.FieldErrors />
				</Form.Fieldset>

				{#if $formData.progressMode === "approved"}
					<Form.Field {form} name="progressPercent">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Approved progress, in percent</Form.Label>
								<Input
									{...props}
									type="number"
									inputmode="numeric"
									min={0}
									max={100}
									step={1}
									class="w-28 font-mono"
									bind:value={$formData.progressPercent}
									disabled={busy}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				{:else if $formData.progressMode === "calculated"}
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{coverage ??
							"No issue selected under this project counts towards progress yet, so readers see no progress figure."}
					</p>
				{/if}
			{/if}

			<Form.Fieldset {form} name="visibility" class="flex flex-col gap-2">
				<Form.Legend>What is shown for this entry</Form.Legend>
				<RowGroup>
					{#each visibilityFields as field (field)}
						{@const effect = effectiveFlag(roadmap.visibility, $formData.visibility, field)}
						<SettingRow {form} name={`visibility.${field}`} label={visibilityLabels[field]} hint={flagEffectLabels[effect]}>
							{#snippet control(props)}
								<Switch
									{...props}
									bind:checked={$formData.visibility[field]}
									disabled={busy}
									class="sm:justify-self-end"
								/>
							{/snippet}
						</SettingRow>
					{/each}
				</RowGroup>
			</Form.Fieldset>

			<Dialog.Footer>
				<Button type="button" variant="secondary" disabled={busy} onclick={onclose}>Cancel</Button>
				<Form.Button disabled={busy || removedMeanwhile}>
					{#if busy}
						Saving…
					{:else if standing.kind === "stale"}
						{target.kind === "add" ? "Add anyway" : "Save mine anyway"}
					{:else}
						{target.kind === "add" ? "Add to roadmap" : "Save changes"}
					{/if}
				</Form.Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
