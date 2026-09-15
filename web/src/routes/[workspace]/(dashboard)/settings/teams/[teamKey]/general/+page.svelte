<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import ColorChoice from "$lib/components/norn/color-choice.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { colorLabels } from "$lib/labels/labels";
	import { useTeamSettings } from "$lib/team/team-settings-context";
	import { settingsFor, teamOf, type TeamSettings } from "$lib/team/team-settings";
	import {
		teamColors,
		teamEstimations,
		teamSettingsSchema,
	} from "$lib/team/team-settings-schema";
	import {
		estimationLabels,
		estimationNotes,
		visibilityLabels,
		visibilityNotes,
		type TeamColor,
		type TeamEstimation,
		type TeamVisibility,
	} from "$lib/team/teams";
	import { teamGeneralPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const settingsFormId = "team-settings-form";
	const visibilities: TeamVisibility[] = ["public", "private"];

	let { data }: PageProps = $props();

	const scope = useTeamSettings();

	const preview = $derived(
		import.meta.env.DEV
			? teamGeneralPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let submitted = $state<TeamSettings | null>(null);
	let archiving = $state(false);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: settingsFormId,
		validators: zod4Client(teamSettingsSchema),
		resetForm: false,
		onUpdated: ({ form: result }) => {
			if (result.message) submitted = null;
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	const settings = $derived<TeamSettings>(
		submitted ??
			$message ??
			preview?.settings ?? { kind: scope.archived ? "archived" : "ready", team: scope.team }
	);
	const team = $derived(teamOf(settings) ?? scope.team);
	const archived = $derived(settings.kind === "archived");
	const readOnly = $derived(scope.readOnly || settings.kind === "read_only");
	const busy = $derived(preview?.busy || $submitting || archiving);
	const locked = $derived(busy || archived || readOnly);

	$effect(() => {
		const { name, description, icon, iconColor, estimation, visibility } = team;
		formData.update(
			(current) => ({ ...current, name, description, icon, iconColor, estimation, visibility }),
			{ taint: false }
		);
	});

	async function setArchived(archive: boolean) {
		archiving = true;

		try {
			const { data: changed, error } = await api.POST(
				archive
					? "/workspaces/{workspaceId}/teams/{teamId}/archive"
					: "/workspaces/{workspaceId}/teams/{teamId}/unarchive",
				{ params: { path: { workspaceId: data.workspace.id, teamId: team.id } } }
			);

			if (changed) {
				submitted = settingsFor(changed);
				await invalidate(keys.workspaceScope(data.workspace.id));

				return;
			}

			if (error) submitted = { kind: "unavailable" };
		} catch {
			submitted = { kind: "unavailable" };
		} finally {
			archiving = false;
		}
	}
</script>

{#if settings.kind === "saved"}
	<Alert.Root variant="success">
		<CircleCheck aria-hidden="true" />
		<Alert.Title>Team saved</Alert.Title>
		<Alert.Description>Everyone sees the change immediately.</Alert.Description>
	</Alert.Root>
{:else if settings.kind === "unavailable"}
	<Alert.Root variant="destructive">
		<CircleX aria-hidden="true" />
		<Alert.Title>That did not save</Alert.Title>
		<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
	</Alert.Root>
{/if}

<section class="flex flex-col gap-4">
	<p class="text-sm leading-normal text-muted-foreground text-pretty">
		How this team appears, and who can find it.
	</p>

	<form id={settingsFormId} method="POST" use:enhance class="flex flex-col gap-4">
		<input type="hidden" name="workspaceId" value={data.workspace.id} />
		<input type="hidden" name="teamId" value={team.id} />

		<Form.Field {form} name="name">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Team name</Form.Label>
					<Input {...props} disabled={locked} bind:value={$formData.name} />
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="icon">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Icon</Form.Label>
					<Input
						{...props}
						disabled={locked}
						bind:value={$formData.icon}
						placeholder="🚀"
						autocomplete="off"
					/>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				An emoji, shown beside the team wherever it appears. Leave it empty for none.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="iconColor">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Colour</Form.Label>
					<ColorChoice
						{...props}
						colors={teamColors}
						labels={colorLabels}
						value={$formData.iconColor}
						name={props.name}
						disabled={locked}
						onpick={(color) => ($formData.iconColor = color as TeamColor)}
					/>
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
						rows={2}
						disabled={locked}
						bind:value={$formData.description}
						placeholder="What this team is responsible for"
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="estimation">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Estimates</Form.Label>
					<Select.Root
						type="single"
						name={props.name}
						value={$formData.estimation}
						disabled={locked}
						onValueChange={(value) => ($formData.estimation = value as TeamEstimation)}
					>
						<Select.Trigger {...props}>
							{estimationLabels[$formData.estimation]}
						</Select.Trigger>
						<Select.Content>
							{#each teamEstimations as estimation (estimation)}
								<Select.Item value={estimation} label={estimationLabels[estimation]}>
									{estimationLabels[estimation]}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				{estimationNotes[$formData.estimation]} Changing this never rewrites an estimate already
				given.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>

		<div class="flex flex-col gap-1">
			<span class="text-sm font-medium text-ink-900">Key</span>
			<TeamKey key={team.key} class="text-md" />
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				The key is permanent. It is stamped on every issue this team has raised —
				{team.key}-1, {team.key}-2 — so changing it would break every reference already quoted
				elsewhere. Archiving never releases it either.
			</p>
		</div>

		<Form.Field {form} name="visibility">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Who can see it</Form.Label>
					<Select.Root
						type="single"
						name={props.name}
						value={$formData.visibility}
						disabled={locked}
						onValueChange={(value) => ($formData.visibility = value as TeamVisibility)}
					>
						<Select.Trigger {...props}>
							{visibilityLabels[$formData.visibility]}
						</Select.Trigger>
						<Select.Content>
							{#each visibilities as visibility (visibility)}
								<Select.Item value={visibility} label={visibilityLabels[visibility]}>
									{visibilityLabels[visibility]}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				{visibilityNotes[$formData.visibility]}
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>
	</form>

	<div>
		<Button type="submit" form={settingsFormId} disabled={locked}>
			{$submitting ? "Saving" : "Save changes"}
		</Button>
	</div>
</section>

{#if archived}
	<section class="flex flex-col gap-4 rounded-lg border border-line-default p-4">
		<div class="flex flex-col gap-1">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Bring this team back</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Restoring {team.name} lets its settings, members and issues change again.
			</p>
		</div>

		<div>
			<Button variant="secondary" disabled={busy || readOnly} onclick={() => setArchived(false)}>
				{archiving ? "Restoring" : "Restore this team"}
			</Button>
		</div>
	</section>
{:else}
	<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
		<div class="flex flex-col gap-1">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Archive this team</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Archiving retires {team.name} without touching its issues. They stay readable and every
				{team.key}-1 style reference keeps resolving. An administrator can bring the team back at
				any time.
			</p>
		</div>

		<div>
			<Button variant="destructive" disabled={busy || readOnly} onclick={() => setArchived(true)}>
				{archiving ? "Archiving" : "Archive team"}
			</Button>
		</div>
	</section>
{/if}
