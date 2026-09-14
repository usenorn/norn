<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import Archive from "@lucide/svelte/icons/archive";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Users from "@lucide/svelte/icons/users";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import ColorChoice from "$lib/components/norn/color-choice.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import TeamMembers from "$lib/team/team-members.svelte";
	import WorkflowStates from "$lib/components/norn/workflow-states.svelte";
	import CycleCadence from "$lib/team/cycle-cadence.svelte";
	import TeamNotifications from "$lib/notifications/team-notifications.svelte";
	import TeamAgents from "$lib/team/team-agents.svelte";
	import TeamIntake from "$lib/team/team-intake.svelte";
	import TeamTriage from "$lib/team/team-triage.svelte";
	import TeamSourceControl from "$lib/source-control/team-source-control.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import type { MemberFailure, TeamRoster } from "$lib/team/members";
	import type { CadenceSetting } from "$lib/cycles/cycles";
	import type { StateList } from "$lib/team/states";
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
	import { colorLabels } from "$lib/labels/labels";
	import TemplateSettings from "$lib/issues/template-settings.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamDetailPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const settingsFormId = "team-settings-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? teamDetailPreviewStates[page.url.searchParams.get("state") ?? ""]
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

	const slug = $derived(page.params.workspace ?? "");
	const settings = $derived<TeamSettings>(
		submitted ?? $message ?? preview?.settings ?? data.settings
	);
	const roster = $derived<TeamRoster>(preview?.roster ?? data.roster);
	const states = $derived<StateList>(preview?.states ?? data.states);
	const cadence = $derived<CadenceSetting>(preview?.cadence ?? data.cadence);
	const triage = $derived(preview?.triage ?? data.triage);
	const intake = $derived(preview?.intake ?? data.intake);
	const notifications = $derived(preview?.notifications ?? data.notifications);
	const failure = $derived<MemberFailure | null>(preview?.failure ?? null);
	const team = $derived(teamOf(settings));
	const archived = $derived(settings.kind === "archived");
	const readOnly = $derived(settings.kind === "read_only");

	const visibilities: TeamVisibility[] = ["public", "private"];

	$effect(() => {
		if (!team) return;

		const { name, description, icon, iconColor, estimation, visibility } = team;
		formData.update(
			(current) => ({ ...current, name, description, icon, iconColor, estimation, visibility }),
			{ taint: false }
		);
	});

	const busy = $derived(preview?.busy || $submitting || archiving);
	const locked = $derived(busy || archived || readOnly);

	async function setArchived(archive: boolean) {
		if (!team) return;

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
				await invalidate(keys.page(page.route.id));

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

<svelte:head>
	<title>{team ? team.name : "Team"} · {data.workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Users class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<a
				href={workspacePath(slug, "/settings/teams")}
				class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Teams
			</a>
			{#if team}
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">{team.name}</h1>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if settings.kind === "loading"}
				<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if settings.kind === "not_found"}
				<div class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">No team here</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no team at this address in {data.workspace.name}, or it is private and you are
						not on it.
					</p>
					<div>
						<Button variant="secondary" size="sm" href={workspacePath(slug, "/settings/teams")}>
							Back to teams
						</Button>
					</div>
				</div>
			{:else if settings.kind === "unavailable" || !team}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this team</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				{#if archived}
					<Alert.Root variant="destructive">
						<Archive aria-hidden="true" />
						<Alert.Title>This team is archived</Alert.Title>
						<Alert.Description>
							Its issues stay readable and {team.key}-1 style references still resolve. Nothing about
							the team can change until it is brought back.
						</Alert.Description>
						<Alert.Action>
							<Button variant="secondary" size="sm" disabled={busy} onclick={() => setArchived(false)}>
								{archiving ? "Restoring" : "Restore this team"}
							</Button>
						</Alert.Action>
					</Alert.Root>
				{/if}

				{#if settings.kind === "saved"}
					<Alert.Root variant="success">
						<CircleCheck aria-hidden="true" />
						<Alert.Title>Team saved</Alert.Title>
						<Alert.Description>Everyone sees the change immediately.</Alert.Description>
					</Alert.Root>
				{/if}

				{#if readOnly}
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>You cannot change this team</Alert.Title>
						<Alert.Description>
							Only workspace administrators can rename a team or change who can see it.
						</Alert.Description>
					</Alert.Root>
				{/if}

				<section class="flex flex-col gap-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">General</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							How this team appears, and who can find it.
						</p>
					</div>

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
								{estimationNotes[$formData.estimation]} Changing this never rewrites an estimate
								already given.
							</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<div class="flex flex-col gap-1">
							<span class="text-sm font-medium text-ink-900">Key</span>
							<TeamKey key={team.key} class="text-md" />
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								The key is permanent. It is stamped on every issue this team has raised —
								{team.key}-1, {team.key}-2 — so changing it would break every reference already
								quoted elsewhere. Archiving never releases it either.
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

				<TemplateSettings
					workspaceId={data.workspace.id}
					workspace={data.workspace.slug}
					teamId={team.id}
					teamName={team.name}
					{locked}
				/>

				<section class="flex flex-col gap-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Members</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Belonging to {data.workspace.name} does not put someone on this team.
						</p>
					</div>

					<TeamMembers
						workspace={{ id: data.workspace.id, name: data.workspace.name, slug }}
						team={{ id: team.id, name: team.name }}
						{roster}
						{failure}
						{readOnly}
						{archived}
						busy={locked}
					/>
				</section>

				<WorkflowStates
					workspaceId={data.workspace.id}
					{team}
					list={states}
					locked={busy || archived || readOnly}
				/>

				<CycleCadence
					workspace={data.workspace}
					{team}
					setting={cadence}
					locked={busy || archived || readOnly}
				/>

				<TeamTriage
					workspace={data.workspace}
					{team}
					setting={triage}
					locked={busy || archived || readOnly}
				/>

				<TeamIntake
					workspace={data.workspace}
					{team}
					setting={intake}
					locked={busy || archived || readOnly}
				/>

				{#if data.sourceControl}
					<TeamSourceControl
						workspace={data.workspace}
						{team}
						rules={data.sourceControl}
						settings={data.sourceControlSettings}
						states={states.kind === "ready" ? states.states : []}
						locked={busy || archived || readOnly}
					/>
				{/if}

				<TeamAgents
					workspace={data.workspace}
					{team}
					settings={data.agents}
					locked={busy || archived || readOnly}
				/>

				<TeamNotifications
					workspace={data.workspace}
					{team}
					setting={notifications}
					locked={busy || archived}
				/>

				{#if !archived}
					<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">Archive this team</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Archiving retires {team.name} without touching its issues. They stay readable and
								every {team.key}-1 style reference keeps resolving. An administrator can bring the
								team back at any time.
							</p>
						</div>

						<div>
							<Button
								variant="destructive"
								disabled={busy || readOnly}
								onclick={() => setArchived(true)}
							>
								{archiving ? "Archiving" : "Archive team"}
							</Button>
						</div>
					</section>
				{/if}
			{/if}
		</div>
	</div>
</div>
