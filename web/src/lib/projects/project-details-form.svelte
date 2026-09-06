<script lang="ts">
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { memberName, type Membership } from "$lib/workspace/members";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import type { Team } from "$lib/team/teams";
	import { projectDetailsSchema } from "./project-details-schema";
	import { teamsChoice, type Project } from "./projects";

	let {
		project,
		members,
		teams,
		locked = false,
		onsave,
		ondirty,
	}: {
		project: Project;
		members: Membership[];
		teams: Team[];
		locked?: boolean;
		onsave: (input: {
			name: string;
			description: string;
			targetOn: string;
			leadAccountId: string;
			teamIds: string[];
		}) => Promise<boolean>;
		ondirty: (dirty: boolean) => void;
	} = $props();

	// svelte-ignore state_referenced_locally
	const held = {
		name: project.name,
		description: project.description,
		targetOn: project.targetOn ?? "",
		leadAccountId: project.leadAccountId ?? "",
		teamIds: project.teamIds ?? [],
	};

	const form = superForm(defaults(held, zod4(projectDetailsSchema)), {
		id: "project-details-form",
		SPA: true,
		validators: zod4Client(projectDetailsSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			await onsave(pending.data);
		},
	});

	const { form: formData, enhance, submitting } = form;

	const people = $derived(members.filter((member) => member.kind !== "agent"));
	const leadName = $derived(
		people.find((member) => member.accountId === $formData.leadAccountId)
	);

	const dirty = $derived(
		$formData.name !== held.name ||
			$formData.description !== held.description ||
			$formData.targetOn !== held.targetOn ||
			$formData.leadAccountId !== held.leadAccountId ||
			$formData.teamIds.join() !== held.teamIds.join()
	);

	$effect(() => {
		ondirty(dirty);
	});
</script>

<form
	id="project-details-form"
	method="POST"
	use:enhance
	class="flex flex-col gap-4 rounded-lg border border-line-strong p-4"
>
	<Form.Field {form} name="name">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Name</Form.Label>
				<Input {...props} bind:value={$formData.name} disabled={locked || $submitting} />
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="description">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Description</Form.Label>
				<Textarea
					{...props}
					rows={6}
					bind:value={$formData.description}
					disabled={locked || $submitting}
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			This is the shared reference. Two paragraphs beats a wiki nobody opens.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<div class="flex flex-col gap-4 sm:flex-row">
		<Form.Field {form} name="targetOn" class="flex-1">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Target date</Form.Label>
					<Input
						{...props}
						type="date"
						bind:value={$formData.targetOn}
						disabled={locked || $submitting}
						class="font-mono"
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="leadAccountId" class="flex-1">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Lead</Form.Label>
					<Select.Root
						type="single"
						name={props.name}
						value={$formData.leadAccountId}
						disabled={locked || $submitting}
						onValueChange={(value) => ($formData.leadAccountId = value)}
					>
						<Select.Trigger {...props} class="w-full">
							{leadName ? memberName(leadName) : "Nobody yet"}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="">Nobody yet</Select.Item>
							{#each people as member (member.accountId)}
								<Select.Item value={member.accountId}>{memberName(member)}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>
	</div>

	{#if teams.length > 0}
		<Form.Field {form} name="teamIds">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Teams</Form.Label>
					<Select.Root
						type="multiple"
						name={props.name}
						value={$formData.teamIds}
						disabled={locked || $submitting}
						onValueChange={(value) => ($formData.teamIds = value)}
					>
						<Select.Trigger {...props} class="w-full">
							{teamsChoice($formData.teamIds, teams)}
						</Select.Trigger>
						<Select.Content>
							{#each teams as team (team.id)}
								<Select.Item value={team.id} label={team.name}>
									<TeamKey key={team.key} />
									{team.name}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				The teams this project serves. It appears under each of them in the sidebar. Pick none and
				it stays the whole workspace's.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>
	{/if}
</form>
