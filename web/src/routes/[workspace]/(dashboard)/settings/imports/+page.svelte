<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Import from "@lucide/svelte/icons/import";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import { startImportSchema } from "$lib/imports/import-schema";
	import {
		counted,
		failureMessage,
		failureTitle,
		importFailure,
		importPath,
		sourceName,
		sourceNote,
		statusLabel,
		statusTone,
		type ImportFailure,
		type ImportsView,
	} from "$lib/imports/imports";
	import { importsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "start-import-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? importsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let localFailure = $state.raw<ImportFailure | null>(null);

	const workspace = $derived(page.data.workspace);
	const view = $derived<ImportsView>(preview?.view ?? data.view);
	const failure = $derived(preview?.failure ?? localFailure);
	const sources = $derived(view.kind === "sources" ? view.sources : []);
	const runs = $derived(view.kind === "sources" ? view.runs : []);

	const form = superForm(defaults(zod4(startImportSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(startImportSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			localFailure = null;

			try {
				const { data: started, error } = await api.POST("/workspaces/{workspaceId}/imports", {
					params: { path: { workspaceId: workspace.id } },
					body: {
						sourceKind: entered.data.sourceKind,
						sourceLabel: entered.data.sourceLabel || undefined,
					},
				});

				if (error) {
					localFailure = importFailure(error);

					return;
				}

				if (!started) {
					localFailure = { kind: "unavailable" };

					return;
				}

				await goto(importPath(workspace.slug, started.id));
			} catch {
				localFailure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const first = sources[0]?.kind;

		if (first && !$formData.sourceKind) {
			formData.update((entered) => ({ ...entered, sourceKind: first }), { taint: false });
		}
	});

	const busy = $derived(preview?.busy || $submitting);
	const chosen = $derived($formData.sourceKind);
</script>

<svelte:head><title>Imports · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-2 border-b border-line-subtle px-4">
		<Import class="size-4 text-muted-foreground" aria-hidden="true" />
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Imports</h1>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>{failureTitle(failure)}</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if view.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not run imports here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the imports</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading imports…</p>
			{:else}
				<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Bring a backlog across</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn reads the source into a copy of its own first, shows you exactly what it would
							create, and applies nothing until you say so. Everything an import makes is recorded,
							so a run can be taken back afterwards.
						</p>
					</div>

					{#if sources.length === 0}
						<Alert.Root variant="muted">
							<CircleAlert aria-hidden="true" />
							<Alert.Title>This instance carries no import sources</Alert.Title>
							<Alert.Description>
								An operator decides which trackers an instance can read from.
							</Alert.Description>
						</Alert.Root>
					{:else}
						<Form.Field {form} name="sourceKind">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Where the backlog is coming from</Form.Label>
									<Select.Root
										type="single"
										value={$formData.sourceKind}
										disabled={busy}
										onValueChange={(value) => ($formData.sourceKind = value)}
									>
										<Select.Trigger {...props}>
											{chosen ? sourceName(chosen) : "Choose a source"}
										</Select.Trigger>
										<Select.Content>
											{#each sources as source (source.kind)}
												<Select.Item value={source.kind} label={sourceName(source.kind)}>
													{sourceName(source.kind)}
												</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								{/snippet}
							</Form.Control>
							{#if chosen}
								<Form.Description class="text-sm text-muted-foreground">
									{sourceNote(chosen)}
								</Form.Description>
							{/if}
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="sourceLabel">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Name this run</Form.Label>
									<Input
										{...props}
										bind:value={$formData.sourceLabel}
										disabled={busy}
										placeholder="Engineering backlog"
										autocomplete="off"
									/>
								{/snippet}
							</Form.Control>
							<Form.Description class="text-sm text-muted-foreground">
								Optional. It only makes this run easier to find later.
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<div>
							<Form.Button disabled={busy}>
								{$submitting ? "Starting…" : "Start an import"}
							</Form.Button>
						</div>
					{/if}
				</form>

				<section class="flex flex-col gap-3">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Runs</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Newest first. A run you started but never finished is still here, and picks up where
							you left it.
						</p>
					</div>

					{#if runs.length === 0}
						<div
							class="flex flex-col items-center gap-2 rounded-lg border border-line-default bg-paper-0 px-6 py-10 text-center"
						>
							<Import class="size-5 text-muted-foreground" aria-hidden="true" />
							<p class="max-w-100 text-sm leading-normal text-muted-foreground text-pretty">
								Nothing has been imported into {workspace.name} yet.
							</p>
						</div>
					{:else}
						<ul class="rounded-lg border border-line-subtle bg-paper-0">
							{#each runs as run (run.id)}
								<li class="border-b border-line-subtle last:border-b-0">
									<a
										href={importPath(workspace.slug, run.id)}
										class="flex flex-col gap-1.5 p-3 motion-control hover:bg-accent"
									>
										<div class="flex flex-wrap items-center gap-2">
											<span class="truncate text-sm text-ink-900">
												{run.sourceLabel || sourceName(run.sourceKind)}
											</span>
											<Tag name={statusLabel(run.status)} color={statusTone(run.status)} />
										</div>
										<p class="text-xs text-muted-foreground">
											{sourceName(run.sourceKind)} · started {onDateAndTime(
												run.createdAt,
												workspace.timezone
											)}
											{#if run.finishedAt}
												· finished {onDateAndTime(run.finishedAt, workspace.timezone)}
											{/if}
										</p>
										<p class="text-xs text-muted-foreground">
											{counted(run.staged, "record read", "records read")} ·
											{counted(run.processed, "record handled", "records handled")}
										</p>
										{#if run.phaseError}
											<p class="text-xs text-destructive">{run.phaseError}</p>
										{/if}
									</a>
								</li>
							{/each}
						</ul>

						{#if view.nextCursor}
							<p class="text-xs text-muted-foreground">
								Older runs are kept. This page shows the most recent {runs.length}.
							</p>
						{/if}
					{/if}
				</section>
			{/if}
		</div>
	</div>
</div>
