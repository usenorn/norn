<script lang="ts" generics="T extends MemoryContentFields">
	import type { FormPath, SuperForm } from "sveltekit-superforms";
	import * as Form from "#lib/components/ui/form/index.js";
	import * as Select from "#lib/components/ui/select/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import { Textarea } from "#lib/components/ui/textarea/index.js";
	import type { Project } from "#lib/projects/projects.js";
	import {
		memoryBasisLabels,
		memoryKindLabels,
		memoryKinds,
		wholeWorkspace,
		type MemoryBasis,
		type MemoryContentFields,
	} from "./memories";

	let {
		form,
		projects,
		disabled = false,
	}: {
		form: SuperForm<T>;
		projects: Project[];
		disabled?: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	const { form: formData } = form;

	const field = (name: keyof MemoryContentFields) => name as FormPath<T>;
	const bases: MemoryBasis[] = ["observation", "inference"];

	const appliesLabel = $derived(
		$formData.appliesTo === wholeWorkspace
			? "The whole workspace"
			: (projects.find((project) => project.id === $formData.appliesTo)?.name ?? "One project")
	);
</script>

<Form.Field {form} name={field("statement")}>
	<Form.Control>
		{#snippet children({ props })}
			<Form.Label>What holds</Form.Label>
			<Textarea {...props} bind:value={$formData.statement} {disabled} rows={2} />
		{/snippet}
	</Form.Control>
	<Form.Description>One statement an agent can act on.</Form.Description>
	<Form.FieldErrors />
</Form.Field>

<div class="grid gap-4 sm:grid-cols-2">
	<Form.Field {form} name={field("kind")}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Kind</Form.Label>
				<Select.Root type="single" name={props.name} bind:value={$formData.kind} {disabled}>
					<Select.Trigger {...props}>{memoryKindLabels[$formData.kind]}</Select.Trigger>
					<Select.Content>
						{#each memoryKinds as kind (kind)}
							<Select.Item value={kind} label={memoryKindLabels[kind]}>{memoryKindLabels[kind]}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name={field("basis")}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Basis</Form.Label>
				<Select.Root type="single" name={props.name} bind:value={$formData.basis} {disabled}>
					<Select.Trigger {...props}>{memoryBasisLabels[$formData.basis]}</Select.Trigger>
					<Select.Content>
						{#each bases as basis (basis)}
							<Select.Item value={basis} label={memoryBasisLabels[basis]}>{memoryBasisLabels[basis]}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>
</div>

<Form.Field {form} name={field("rationale")}>
	<Form.Control>
		{#snippet children({ props })}
			<Form.Label>Why</Form.Label>
			<Textarea {...props} bind:value={$formData.rationale} {disabled} rows={3} />
		{/snippet}
	</Form.Control>
	<Form.FieldErrors />
</Form.Field>

<Form.Field {form} name={field("conditions")}>
	<Form.Control>
		{#snippet children({ props })}
			<Form.Label>Holds when</Form.Label>
			<Input {...props} bind:value={$formData.conditions} {disabled} autocomplete="off" />
		{/snippet}
	</Form.Control>
	<Form.Description>Leave it empty when it always holds, or say when, such as “local prototypes only”.</Form.Description>
	<Form.FieldErrors />
</Form.Field>

<div class="grid gap-4 sm:grid-cols-2">
	<Form.Field {form} name={field("appliesTo")}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Applies to</Form.Label>
				<Select.Root type="single" name={props.name} bind:value={$formData.appliesTo} {disabled}>
					<Select.Trigger {...props}>{appliesLabel}</Select.Trigger>
					<Select.Content>
						<Select.Item value={wholeWorkspace} label="The whole workspace">The whole workspace</Select.Item>
						{#each projects as project (project.id)}
							<Select.Item value={project.id} label={project.name}>{project.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name={field("component")}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Component</Form.Label>
				<Input {...props} bind:value={$formData.component} {disabled} autocomplete="off" />
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>
</div>

<div class="grid gap-4 sm:grid-cols-2">
	<Form.Field {form} name={field("effectiveFrom")}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Holds from</Form.Label>
				<Input {...props} type="date" bind:value={$formData.effectiveFrom} {disabled} />
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name={field("effectiveUntil")}>
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Holds until</Form.Label>
				<Input {...props} type="date" bind:value={$formData.effectiveUntil} {disabled} />
			{/snippet}
		</Form.Control>
		<Form.Description>Leave it empty when it holds until replaced.</Form.Description>
		<Form.FieldErrors />
	</Form.Field>
</div>
