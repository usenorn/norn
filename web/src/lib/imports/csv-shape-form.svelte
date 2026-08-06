<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { csvShapeSchema } from "./import-schema";
	import { readCatalogue, stageRun } from "./steps";
	import {
		csvColumnTargetLabel,
		csvColumnTargets,
		csvDelimiters,
		csvSettings,
		confidenceNote,
		importFailure,
		type ImportCatalogue,
		type ImportFailure,
		type ImportRun,
		type ImportRunView,
	} from "./imports";

	let {
		workspaceId,
		run,
		catalogue,
		onview,
		onadvanced,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		catalogue: ImportCatalogue;
		onview: (view: ImportRunView) => void;
		onadvanced: () => Promise<void>;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	const formId = "import-shape-form";
	const defaultTeamKey = "CSV";
	const defaultTeamName = "Imported";

	let intent = $state<"reread" | "stage">("stage");

	const held = $derived(csvSettings(run));

	const form = superForm(defaults(zod4(csvShapeSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(csvShapeSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			onfailure(null);

			try {
				const { data: saved, error } = await api.PUT(
					"/workspaces/{workspaceId}/imports/{importRunId}/source",
					{
						params: { path: { workspaceId, importRunId: run.id } },
						body: {
							settings: {
								objectKey: held.objectKey,
								fileName: held.fileName,
								delimiter: entered.data.delimiter,
								header: entered.data.header,
								teamKey: entered.data.teamKey,
								teamName: entered.data.teamName,
								columns: entered.data.columns,
							},
							unknownReferences: run.unknownReferences,
						},
					}
				);

				if (error) {
					onfailure(importFailure(error));

					return;
				}

				if (!saved) {
					onfailure({ kind: "unavailable" });

					return;
				}

				if (intent === "reread") {
					const result = await readCatalogue(workspaceId, saved);

					if (result.ok) {
						onview(result.view);
					} else {
						onfailure(result.failure);
					}

					return;
				}

				const staged = await stageRun(workspaceId, run.id);

				if (staged) {
					onfailure(staged);

					return;
				}

				await onadvanced();
			} catch {
				onfailure({ kind: "unavailable" });
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const decided = new Map(held.columns.map((column) => [column.index, column.target]));

		formData.update(
			(entered) => ({
				...entered,
				delimiter: held.delimiter,
				header: held.header,
				teamKey: held.teamKey || defaultTeamKey,
				teamName: held.teamName || defaultTeamName,
				columns: catalogue.columns.map((column) => ({
					index: column.index,
					target: decided.get(column.index) ?? column.proposed ?? "ignore",
				})),
			}),
			{ taint: false }
		);
	});

	function retarget(index: number, target: string) {
		formData.update((entered) => ({
			...entered,
			columns: entered.columns.map((column) =>
				column.index === index ? { ...column, target } : column
			),
		}));
	}

	function targetAt(index: number): string {
		return $formData.columns.find((column) => column.index === index)?.target ?? "ignore";
	}

	function submitAs(next: "reread" | "stage") {
		intent = next;
	}

	const delimiter = $derived(
		csvDelimiters.find((option) => option.value === $formData.delimiter) ?? csvDelimiters[0]
	);
	const importable = $derived(
		$formData.columns.some((column) => column.target === "title") ||
			catalogue.columns.length === 0
	);
</script>

<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">What the columns hold</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Norn read the head of the file and proposed a meaning for each column. Correct anything it
			got wrong. A column left as not imported is read and thrown away.
		</p>
	</div>

	{#if catalogue.notes.length > 0}
		<ul class="flex flex-col gap-1.5 rounded-md border border-line-subtle bg-paper-0 p-3">
			{#each catalogue.notes as note, index (index)}
				<li class="text-sm leading-normal text-muted-foreground text-pretty">{note}</li>
			{/each}
		</ul>
	{/if}

	<Form.Field {form} name="delimiter">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>What separates the columns</Form.Label>
				<Select.Root
					type="single"
					value={$formData.delimiter}
					disabled={$submitting}
					onValueChange={(value) => ($formData.delimiter = value)}
				>
					<Select.Trigger {...props}>{delimiter.label}</Select.Trigger>
					<Select.Content>
						{#each csvDelimiters as option (option.label)}
							<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			Change this and re-read the file: with the wrong separator every row arrives as one column.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="header">
		<Form.Control>
			{#snippet children({ props })}
				<div class="flex items-start gap-2">
					<Checkbox
						{...props}
						disabled={$submitting}
						checked={$formData.header}
						onCheckedChange={(checked) => ($formData.header = checked === true)}
					/>
					<Form.Label>The first row names the columns</Form.Label>
				</div>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			Leave this off and the first row is imported as an issue like any other, and the columns are
			chosen by position instead.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="columns">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Columns</Form.Label>
				<div {...props} class="flex flex-col gap-2">
					{#each catalogue.columns as column (column.index)}
						{@const note = confidenceNote(column.confidence)}
						<div
							class="flex flex-wrap items-center gap-2 rounded-md border border-line-subtle px-3 py-2"
						>
							<div class="flex min-w-40 flex-1 flex-col gap-0.5">
								<span class="text-sm text-ink-900">
									{column.header || `Column ${column.index + 1}`}
								</span>
								{#if note}
									<span class="text-xs text-muted-foreground">{note}</span>
								{/if}
							</div>
							<Select.Root
								type="single"
								value={targetAt(column.index)}
								disabled={$submitting}
								onValueChange={(value) => retarget(column.index, value)}
							>
								<Select.Trigger class="w-56" aria-label="What column {column.index + 1} holds">
									{csvColumnTargetLabel(targetAt(column.index))}
								</Select.Trigger>
								<Select.Content>
									{#each csvColumnTargets as target (target.value)}
										<Select.Item value={target.value} label={target.label}>
											{target.label}
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
					{:else}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Nothing readable came back from the head of this file. Check the separator above and
							re-read it.
						</p>
					{/each}
				</div>
			{/snippet}
		</Form.Control>
		{#if !importable}
			<Form.Description class="text-sm text-muted-foreground">
				Nothing is set to be the title yet. An issue with no title is an issue that is skipped.
			</Form.Description>
		{/if}
		<Form.FieldErrors />
	</Form.Field>

	<div class="flex flex-col gap-4 rounded-md border border-line-subtle p-3">
		<div class="flex flex-col gap-1">
			<h3 class="text-sm font-medium tracking-snug text-ink-900">The team these rows stand for</h3>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				A file of rows names no team of its own. One team stands in for the whole file, and you map
				it onto a real team in the next step.
			</p>
		</div>

		<Form.Field {form} name="teamKey">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Key</Form.Label>
					<Input
						{...props}
						bind:value={$formData.teamKey}
						disabled={$submitting}
						autocapitalize="characters"
						autocomplete="off"
						spellcheck="false"
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="teamName">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Name</Form.Label>
					<Input
						{...props}
						bind:value={$formData.teamName}
						disabled={$submitting}
						autocomplete="off"
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>
	</div>

	<div class="flex flex-wrap gap-2">
		<Form.Button disabled={$submitting} onclick={() => submitAs("stage")}>
			{$submitting && intent === "stage" ? "Starting the read…" : "Read this file into Norn"}
		</Form.Button>
		<Button
			type="submit"
			form={formId}
			variant="secondary"
			disabled={$submitting}
			onclick={() => submitAs("reread")}
		>
			{$submitting && intent === "reread" ? "Re-reading…" : "Re-read the columns"}
		</Button>
	</div>
</form>
