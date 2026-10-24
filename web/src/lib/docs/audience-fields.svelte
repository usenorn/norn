<script lang="ts" generics="T extends { visibility: DocVisibility; projectId: string }">
	import type { FormPath, SuperForm } from "sveltekit-superforms";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import type { Project } from "$lib/projects/projects";
	import { docAudienceLabels, docVisibilities, docVisibilityLabels, type DocVisibility } from "./docs";

	let {
		form,
		projects,
		disabled = false,
	}: {
		form: SuperForm<T>;
		projects: Project[];
		disabled?: boolean;
	} = $props();

	const { form: formData } = $derived(form);

	const visibility = "visibility" as FormPath<T>;
	const projectId = "projectId" as FormPath<T>;

	const chosenProject = $derived(projects.find((project) => project.id === $formData.projectId));
</script>

<Form.Field {form} name={visibility}>
	<Form.Control>
		{#snippet children({ props })}
			<Form.Label>Who reads it</Form.Label>
			<Select.Root type="single" name={props.name} bind:value={$formData.visibility} {disabled}>
				<Select.Trigger {...props}>{docVisibilityLabels[$formData.visibility]}</Select.Trigger>
				<Select.Content>
					{#each docVisibilities as option (option)}
						<Select.Item value={option} label={docVisibilityLabels[option]} disabled={option === "project" && projects.length === 0}>
							{docVisibilityLabels[option]}
						</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/snippet}
	</Form.Control>
	<Form.Description>{docAudienceLabels[$formData.visibility]}</Form.Description>
	<Form.FieldErrors />
</Form.Field>

{#if $formData.visibility === "project"}
	<Form.Field {form} name={projectId}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Project</Form.Label>
				<Select.Root type="single" name={props.name} bind:value={$formData.projectId} {disabled}>
					<Select.Trigger {...props}>{chosenProject?.name ?? "Choose a project"}</Select.Trigger>
					<Select.Content>
						{#each projects as project (project.id)}
							<Select.Item value={project.id} label={project.name}>{project.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>
{:else}
	<input type="hidden" name="projectId" value="" />
{/if}
