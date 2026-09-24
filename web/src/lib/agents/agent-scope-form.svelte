<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import type { Project } from "$lib/projects/projects";
	import { agentScopeLabels, agentScopeHints, agentScopes, type AgentScope } from "./agents";
	import { agentScopeSchema } from "./agent-scope-schema";

	let {
		scope,
		projectId,
		projects,
		mayOpenToWorkspace,
		locked = false,
		onsave,
	}: {
		scope: AgentScope;
		projectId: string;
		projects: Project[];
		mayOpenToWorkspace: boolean;
		locked?: boolean;
		onsave: (input: { scope: AgentScope; projectId: string }) => Promise<boolean>;
	} = $props();

	// svelte-ignore state_referenced_locally
	const opened = { scope, projectId };

	let held = $state(opened);

	const form = superForm(defaults(opened, zod4(agentScopeSchema)), {
		id: "agent-scope-form",
		SPA: true,
		validators: zod4Client(agentScopeSchema),
		resetForm: false,
		invalidateAll: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			if (await onsave(pending.data)) held = { ...pending.data };
		},
	});

	const { form: formData, enhance, submitting } = form;

	const offered = $derived(
		agentScopes.filter(
			(offer) =>
				(offer !== "workspace" || mayOpenToWorkspace || scope === "workspace") &&
				(offer !== "project" || projects.length > 0 || scope === "project")
		)
	);

	const dirty = $derived(
		$formData.scope !== held.scope || $formData.projectId !== held.projectId
	);
</script>

<form
	id="agent-scope-form"
	method="POST"
	use:enhance
	class="flex flex-col gap-4 rounded-lg border border-line-strong p-4"
>
	<Form.Fieldset {form} name="scope">
		<Form.Legend>Who may hand it work</Form.Legend>
		<div class="flex flex-col gap-2">
			{#each offered as offer (offer)}
				<div class="flex items-start gap-2">
					<input
						id={`agent-scope-${offer}`}
						type="radio"
						value={offer}
						disabled={locked || $submitting}
						checked={$formData.scope === offer}
						onchange={() =>
							formData.update((entered) => ({
								...entered,
								scope: offer,
								projectId: offer === "project" ? entered.projectId : "",
							}))}
						class="mt-0.5 size-3.5 accent-ink-900"
					/>
					<label for={`agent-scope-${offer}`} class="flex flex-col gap-0.5">
						<span class="text-sm leading-normal text-ink-900">{agentScopeLabels[offer]}</span>
						<span class="text-xs leading-normal text-ink-400">{agentScopeHints[offer]}</span>
					</label>
				</div>
			{/each}
		</div>
		<Form.FieldErrors />
	</Form.Fieldset>

	{#if $formData.scope === "project"}
		<Form.Field {form} name="projectId">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Project</Form.Label>
					<select
						{...props}
						bind:value={$formData.projectId}
						disabled={locked || $submitting}
						class="h-9 max-w-sm rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
					>
						<option value="">Choose a project</option>
						{#each projects as project (project.id)}
							<option value={project.id}>{project.name}</option>
						{/each}
					</select>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>
	{/if}

	<div class="flex justify-end">
		<Button type="submit" size="sm" disabled={locked || $submitting || !dirty}>
			{$submitting ? "Saving…" : "Save scope"}
		</Button>
	</div>
</form>
