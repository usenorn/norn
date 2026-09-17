<script lang="ts">
	import type { SuperForm } from "sveltekit-superforms";
	import Check from "@lucide/svelte/icons/check";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import ColorChoice from "$lib/components/norn/color-choice.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import {
		colorLabels,
		labelColors,
		labelFailureMessage,
		refusedLabelFailure,
		type LabelColor,
		type LabelFailure,
		type LabelGroup,
	} from "./labels";
	import type { LabelInput } from "./label-schema";
	import type { Team } from "$lib/team/teams";

	const newGroup = "__new__";
	const minimumName = 2;

	let {
		form,
		formId,
		workspaceId,
		groups,
		teams,
		editing,
		submitting,
		ongroupcreated,
		oncancel,
	}: {
		form: SuperForm<LabelInput, LabelFailure>;
		formId: string;
		workspaceId: string;
		groups: LabelGroup[];
		teams: Team[];
		editing: boolean;
		submitting: boolean;
		ongroupcreated: (group: LabelGroup) => void;
		oncancel: () => void;
	} = $props();

	const formData = $derived(form.form);

	let namingGroup = $state(false);
	let groupName = $state("");
	let creatingGroup = $state(false);
	let groupError = $state("");

	const tooShort = $derived($formData.name.trim().length < minimumName);
	const busy = $derived(submitting || creatingGroup);

	function startNamingGroup() {
		namingGroup = true;
		groupName = "";
		groupError = "";
	}

	function stopNamingGroup() {
		namingGroup = false;
		groupName = "";
		groupError = "";
	}

	async function createGroup() {
		const name = groupName.trim();

		if (!name) {
			groupError = "Enter a group name.";

			return;
		}

		creatingGroup = true;
		groupError = "";

		try {
			const { data, error, response } = await api.POST("/workspaces/{workspaceId}/label-groups", {
				params: { path: { workspaceId } },
				body: { name },
			});

			if (error || !data) {
				groupError = labelFailureMessage(refusedLabelFailure(error, response?.status ?? 0));

				return;
			}

			ongroupcreated(data);
			$formData.groupId = data.id;
			stopNamingGroup();
		} catch {
			groupError = labelFailureMessage({ kind: "unavailable" });
		} finally {
			creatingGroup = false;
		}
	}
</script>

<div class="flex flex-col gap-3">
	<Form.Field {form} name="name">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label class="sr-only">Name</Form.Label>
				<Input
					{...props}
					disabled={busy}
					placeholder="Label name"
					bind:value={$formData.name}
					class="font-mono tracking-caps uppercase"
				/>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="description">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label class="sr-only">Description</Form.Label>
				<Input
					{...props}
					disabled={busy}
					placeholder="What it is for (optional)"
					bind:value={$formData.description}
				/>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="color">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>
					<Eyebrow>Colour</Eyebrow>
				</Form.Label>
				<ColorChoice
					{...props}
					colors={labelColors}
					labels={colorLabels}
					value={$formData.color}
					name={props.name}
					disabled={busy}
					onpick={(color) => ($formData.color = color as LabelColor)}
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-xs text-muted-foreground">
			Status owns red, amber and green, so they are not offered here.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="groupId">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>
					<Eyebrow>Group</Eyebrow>
				</Form.Label>
				{#if namingGroup}
					<div class="flex flex-col gap-2">
						<div class="flex items-center gap-2">
							<Input
								bind:value={groupName}
								disabled={creatingGroup}
								placeholder="Severity"
								aria-label="New group name"
								aria-invalid={groupError ? "true" : undefined}
							/>
							<Button
								type="button"
								size="icon-sm"
								disabled={creatingGroup}
								aria-label="Create group"
								onclick={createGroup}
							>
								<Check aria-hidden="true" />
							</Button>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								disabled={creatingGroup}
								onclick={stopNamingGroup}
							>
								Cancel
							</Button>
						</div>
						{#if groupError}
							<p class="text-sm text-destructive">{groupError}</p>
						{/if}
						<input type="hidden" name={props.name} value={$formData.groupId} />
					</div>
				{:else}
					<Select.Root
						type="single"
						name={props.name}
						value={$formData.groupId}
						disabled={busy}
						onValueChange={(value) => {
							if (value === newGroup) {
								startNamingGroup();

								return;
							}

							$formData.groupId = value;
						}}
					>
						<Select.Trigger {...props}>
							{groups.find((group) => group.id === $formData.groupId)?.name ?? "No group"}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="" label="No group">No group</Select.Item>
							{#each groups as group (group.id)}
								<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
							{/each}
							<Select.Item value={newGroup} label="New group…">New group…</Select.Item>
						</Select.Content>
					</Select.Root>
				{/if}
			{/snippet}
		</Form.Control>
		<Form.Description class="text-xs text-muted-foreground">
			An issue carries at most one label from a group.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	{#if !editing}
		<Form.Field {form} name="teamId">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>
						<Eyebrow>Scope</Eyebrow>
					</Form.Label>
					<Select.Root
						type="single"
						name={props.name}
						value={$formData.teamId}
						disabled={busy}
						onValueChange={(value) => ($formData.teamId = value)}
					>
						<Select.Trigger {...props}>
							{teams.find((team) => team.id === $formData.teamId)?.name ??
								"Every team in this workspace"}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="" label="Every team in this workspace">
								Every team in this workspace
							</Select.Item>
							{#each teams as team (team.id)}
								<Select.Item value={team.id} label={team.name}>{team.name}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-xs text-muted-foreground">
				Scope is permanent. To widen or narrow a label later, merge it into one with the scope you
				want.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>
	{/if}

	<div class="flex items-center gap-2 pt-0.5">
		<Eyebrow class="flex-1">Preview</Eyebrow>
		<Tag name={$formData.name.trim() || "Label"} color={$formData.color} />
	</div>
</div>

<div class="flex items-center justify-end gap-2 pt-3">
	<Button type="button" variant="ghost" size="sm" disabled={busy} onclick={oncancel}>Cancel</Button>
	<Button type="submit" form={formId} size="sm" disabled={busy || tooShort}>
		{submitting ? "Saving" : editing ? "Save" : "Create"}
	</Button>
</div>
