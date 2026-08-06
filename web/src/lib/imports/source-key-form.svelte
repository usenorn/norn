<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { linearKeySchema } from "./import-schema";
	import { readCatalogue } from "./steps";
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
		settings,
		askPolicy = true,
		submitLabel,
		pendingLabel,
		oncancel,
		onview,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		settings: Record<string, unknown>;
		askPolicy?: boolean;
		submitLabel: string;
		pendingLabel: string;
		oncancel?: () => void;
		onview: (view: ImportRunView) => void;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	const formId = "import-key-form";

	const form = superForm(defaults(zod4(linearKeySchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(linearKeySchema),
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
							apiKey: entered.data.apiKey,
							settings,
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

	$effect(() => {
		const policy = run.unknownReferences;

		formData.update((entered) => ({ ...entered, unknownReferences: policy }), { taint: false });
	});

	const chosen = $derived(
		unknownPolicies.find((policy) => policy.value === $formData.unknownReferences)
	);
</script>

<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
	<Form.Field {form} name="apiKey">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Linear personal API key</Form.Label>
				<Input
					{...props}
					bind:value={$formData.apiKey}
					disabled={$submitting}
					type="password"
					placeholder="lin_api_…"
					autocomplete="off"
					spellcheck="false"
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			Create one in Linear under Settings, Security and access. The key is stored encrypted, is
			never shown again, and is read only while this run is drawing from the source.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	{#if askPolicy}
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
	{/if}

	<div class="flex flex-wrap gap-2">
		<Form.Button disabled={$submitting}>
			{$submitting ? pendingLabel : submitLabel}
		</Form.Button>
		{#if oncancel}
			<Button type="button" variant="ghost" disabled={$submitting} onclick={oncancel}>
				Keep the key it has
			</Button>
		{/if}
	</div>
</form>
