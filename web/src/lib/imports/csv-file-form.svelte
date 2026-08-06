<script lang="ts">
	import { defaults, fileProxy, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { csvFileSchema } from "./import-schema";
	import { readCatalogue, uploadFile } from "./steps";
	import {
		importFailure,
		unknownPolicies,
		type ImportFailure,
		type ImportRun,
		type ImportRunView,
	} from "./imports";

	let {
		workspaceId,
		run,
		onview,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		onview: (view: ImportRunView) => void;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	const formId = "import-file-form";

	const form = superForm(defaults(zod4(csvFileSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(csvFileSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || !entered.data.file) return;

			onfailure(null);

			try {
				const sent = await uploadFile(workspaceId, run.id, entered.data.file);

				if (!sent.ok) {
					onfailure(sent.failure);

					return;
				}

				const { data: saved, error } = await api.PUT(
					"/workspaces/{workspaceId}/imports/{importRunId}/source",
					{
						params: { path: { workspaceId, importRunId: run.id } },
						body: {
							settings: { objectKey: sent.file.objectKey, fileName: sent.file.fileName },
							unknownReferences: entered.data.unknownReferences,
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

				const result = await readCatalogue(workspaceId, saved);

				if (result.ok) {
					onview(result.view);
				} else {
					onfailure(result.failure);
				}
			} catch {
				onfailure({ kind: "unavailable" });
			}
		},
	});
	const { form: formData, enhance, submitting } = form;
	const chosenFile = fileProxy(form, "file");

	$effect(() => {
		const policy = run.unknownReferences;

		formData.update((entered) => ({ ...entered, unknownReferences: policy }), { taint: false });
	});

	const chosen = $derived(
		unknownPolicies.find((policy) => policy.value === $formData.unknownReferences)
	);
</script>

<form
	method="POST"
	id={formId}
	enctype="multipart/form-data"
	use:enhance
	class="flex flex-col gap-5"
>
	<Form.Field {form} name="file">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>The file to read</Form.Label>
				<Input
					{...props}
					type="file"
					accept=".csv,.tsv,text/csv,text/tab-separated-values,text/plain"
					disabled={$submitting}
					bind:files={$chosenFile}
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			One row per issue, with the columns named on the first line if the file has them. The file is
			held only for this run and is read as the rows are staged.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="unknownReferences">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>When a row names something that never arrived</Form.Label>
				<Select.Root
					type="single"
					value={$formData.unknownReferences}
					disabled={$submitting}
					onValueChange={(value) =>
						($formData.unknownReferences = value as typeof $formData.unknownReferences)}
				>
					<Select.Trigger {...props}>{chosen?.label ?? "Choose what happens"}</Select.Trigger>
					<Select.Content>
						{#each unknownPolicies as policy (policy.value)}
							<Select.Item value={policy.value} label={policy.label}>{policy.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/snippet}
		</Form.Control>
		{#if chosen}
			<Form.Description class="text-sm text-muted-foreground">{chosen.note}</Form.Description>
		{/if}
		<Form.FieldErrors />
	</Form.Field>

	<div>
		<Form.Button disabled={$submitting}>
			{$submitting ? "Uploading…" : "Upload and read the columns"}
		</Form.Button>
	</div>
</form>
