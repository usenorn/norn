<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { api } from "$lib/api";
	import { linearScopeSchema } from "./import-schema";
	import { stageRun } from "./steps";
	import {
		importFailure,
		linearSettings,
		type ImportCatalogue,
		type ImportFailure,
		type ImportRun,
	} from "./imports";

	let {
		workspaceId,
		run,
		catalogue,
		onadvanced,
		onfailure,
	}: {
		workspaceId: string;
		run: ImportRun;
		catalogue: ImportCatalogue;
		onadvanced: () => Promise<void>;
		onfailure: (failure: ImportFailure | null) => void;
	} = $props();

	const formId = "import-scope-form";

	const form = superForm(defaults(zod4(linearScopeSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(linearScopeSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			onfailure(null);

			try {
				const { error } = await api.PUT(
					"/workspaces/{workspaceId}/imports/{importRunId}/source",
					{
						params: { path: { workspaceId, importRunId: run.id } },
						body: {
							settings: { teamIds: entered.data.teamIds },
							unknownReferences: run.unknownReferences,
						},
					}
				);

				if (error) {
					onfailure(importFailure(error));

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
		const teamIds = linearSettings(run).teamIds;

		if (teamIds.length > 0) {
			formData.update((entered) => ({ ...entered, teamIds }), { taint: false });
		}
	});

	function toggle(key: string, checked: boolean) {
		const next = new Set($formData.teamIds);

		if (checked) {
			next.add(key);
		} else {
			next.delete(key);
		}

		formData.update((entered) => ({ ...entered, teamIds: [...next] }));
	}

	function all(checked: boolean) {
		formData.update((entered) => ({
			...entered,
			teamIds: checked ? catalogue.scopes.map((scope) => scope.key) : [],
		}));
	}

	const every = $derived(
		catalogue.scopes.length > 0 &&
			catalogue.scopes.every((scope) => $formData.teamIds.includes(scope.key))
	);
</script>

<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Which teams to read</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Everything is narrowed by this: only issues, projects, cycles and labels belonging to the
			teams you pick are read. A team you leave out costs nothing and can be brought across by a
			later run.
		</p>
	</div>

	{#if catalogue.scopes.length === 0}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			This key can see no teams. Check that it belongs to an account with access to the workspace
			you meant, then replace it above.
		</p>
	{:else}
		<Form.Field {form} name="teamIds">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Teams</Form.Label>
					<div {...props} class="flex flex-col gap-1.5 rounded-md border border-line-subtle p-3">
						<div class="flex items-center gap-2 border-b border-line-subtle pb-2">
							<Checkbox
								id={`scope-all-${run.id}`}
								disabled={$submitting}
								checked={every}
								onCheckedChange={(checked) => all(checked === true)}
							/>
							<label
								for={`scope-all-${run.id}`}
								class="text-sm leading-normal font-medium tracking-snug text-ink-900"
							>
								Every team this key can see
							</label>
						</div>

						{#each catalogue.scopes as scope (scope.key)}
							<div class="flex items-start gap-2">
								<Checkbox
									id={`scope-${scope.key}`}
									disabled={$submitting}
									checked={$formData.teamIds.includes(scope.key)}
									onCheckedChange={(checked) => toggle(scope.key, checked === true)}
								/>
								<label for={`scope-${scope.key}`} class="text-sm leading-normal text-ink-600">
									{scope.name}
									{#if scope.detail}
										<span class="font-mono text-xs text-muted-foreground">{scope.detail}</span>
									{/if}
								</label>
							</div>
						{/each}
					</div>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>
	{/if}

	{#if catalogue.notes.length > 0}
		<div class="flex flex-col gap-2 rounded-md border border-line-subtle bg-paper-0 p-3">
			<h3 class="text-sm font-medium tracking-snug text-ink-900">What will not come across whole</h3>
			<ul class="flex flex-col gap-1.5">
				{#each catalogue.notes as note, index (index)}
					<li class="text-sm leading-normal text-muted-foreground text-pretty">{note}</li>
				{/each}
			</ul>
		</div>
	{/if}

	<div>
		<Form.Button disabled={$submitting || catalogue.scopes.length === 0}>
			{$submitting ? "Starting the read…" : "Read these teams into Norn"}
		</Form.Button>
	</div>
</form>
