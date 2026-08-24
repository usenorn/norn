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
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import WorkflowStates from "$lib/components/norn/workflow-states.svelte";
	import CycleCadence from "$lib/team/cycle-cadence.svelte";
	import TeamNotifications from "$lib/notifications/team-notifications.svelte";
	import TeamAgents from "$lib/team/team-agents.svelte";
	import TeamTriage from "$lib/team/team-triage.svelte";
	import TeamSourceControl from "$lib/source-control/team-source-control.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import {
		initialsOf,
		memberFailureMessage,
		membersOf,
		rosterFor,
		type MemberFailure,
		type TeamRoster,
	} from "$lib/team/members";
	import type { CadenceSetting } from "$lib/cycles/cycles";
	import type { StateList } from "$lib/team/states";
	import { settingsFor, teamOf, type TeamSettings } from "$lib/team/team-settings";
	import { teamSettingsSchema } from "$lib/team/team-settings-schema";
	import { visibilityLabels, visibilityNotes, type TeamVisibility } from "$lib/team/teams";
	import {
		memberName,
		searchDebounceMs,
		type Membership,
	} from "$lib/workspace/members";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamDetailPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const settingsFormId = "team-settings-form";
	const candidateLimit = 8;

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? teamDetailPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let submitted = $state<TeamSettings | null>(null);
	let submittedRoster = $state<TeamRoster | null>(null);
	let memberFailure = $state<MemberFailure | null>(null);
	let archiving = $state(false);
	let removing = $state("");

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
	const roster = $derived<TeamRoster>(submittedRoster ?? preview?.roster ?? data.roster);
	const states = $derived<StateList>(preview?.states ?? data.states);
	const cadence = $derived<CadenceSetting>(preview?.cadence ?? data.cadence);
	const triage = $derived(preview?.triage ?? data.triage);
	const notifications = $derived(preview?.notifications ?? data.notifications);
	const failure = $derived<MemberFailure | null>(preview?.failure ?? memberFailure);
	const team = $derived(teamOf(settings));
	const archived = $derived(settings.kind === "archived");
	const readOnly = $derived(settings.kind === "read_only");

	const members = $derived(membersOf(roster));
	const visibilities: TeamVisibility[] = ["public", "private"];

	let candidateQuery = $state("");
	let candidates = $state<Membership[]>([]);
	let searching = $state(false);
	let adding = $state("");
	let candidateDebounce: ReturnType<typeof setTimeout> | undefined;

	$effect(() => () => clearTimeout(candidateDebounce));

	async function findCandidates(query: string) {
		if (!team || !query) {
			candidates = [];

			return;
		}

		searching = true;

		try {
			const { data: page } = await api.GET("/workspaces/{workspaceId}/members", {
				params: {
					path: { workspaceId: data.workspace.id },
					query: { query, limit: candidateLimit },
				},
			});

			candidates = (page?.members ?? []).filter(
				(candidate) => !members.some((member) => member.accountId === candidate.accountId)
			);
		} catch {
			candidates = [];
		} finally {
			searching = false;
		}
	}

	function searchCandidates(value: string) {
		candidateQuery = value;
		clearTimeout(candidateDebounce);
		candidateDebounce = setTimeout(() => findCandidates(value), searchDebounceMs);
	}

	async function addMember(accountId: string) {
		if (!team) return;

		adding = accountId;
		memberFailure = null;

		try {
			const { data: added, error } = await api.POST(
				"/workspaces/{workspaceId}/teams/{teamId}/members",
				{
					params: { path: { workspaceId: data.workspace.id, teamId: team.id } },
					body: { accountId },
				}
			);

			if (added) {
				submittedRoster = { kind: "added", members: [...members, added], member: added };
				candidates = candidates.filter((candidate) => candidate.accountId !== accountId);
				await invalidate(keys.page(page.route.id));

				return;
			}

			if (error?.status === 403) {
				memberFailure = { kind: "forbidden" };

				return;
			}

			if (error && "code" in error && error.code === "team_member_exists") {
				memberFailure = { kind: "already_member" };

				return;
			}

			if (error?.status === 404) {
				memberFailure = { kind: "not_in_workspace" };

				return;
			}

			memberFailure = { kind: "unavailable" };
		} catch {
			memberFailure = { kind: "unavailable" };
		} finally {
			adding = "";
		}
	}

	$effect(() => {
		if (!team) return;

		const { name, visibility } = team;
		formData.update((current) => ({ ...current, name, visibility }), { taint: false });
	});

	const busy = $derived(
		preview?.busy || $submitting || archiving || removing !== "" || adding !== ""
	);
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

	async function removeMember(accountId: string) {
		if (!team) return;

		removing = accountId;
		memberFailure = null;

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/teams/{teamId}/members/{accountId}",
				{ params: { path: { workspaceId: data.workspace.id, teamId: team.id, accountId } } }
			);

			if (error) {
				memberFailure = error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

				return;
			}

			submittedRoster = rosterFor(members.filter((member) => member.accountId !== accountId));
			await invalidate(keys.page(page.route.id));
		} catch {
			memberFailure = { kind: "unavailable" };
		} finally {
			removing = "";
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

	<div class="relative flex-1 overflow-auto">
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

				<section class="flex flex-col gap-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Members</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Belonging to {data.workspace.name} does not put someone on this team.
						</p>
					</div>

					{#if failure}
						<Alert.Root variant="destructive">
							<CircleX aria-hidden="true" />
							<Alert.Title>That did not work</Alert.Title>
							<Alert.Description>{memberFailureMessage(failure)}</Alert.Description>
						</Alert.Root>
					{/if}

					{#if roster.kind === "loading"}
						<div class="h-20 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
					{:else if roster.kind === "unavailable"}
						<p class="text-sm leading-normal text-muted-foreground">
							We could not load who is on this team.
						</p>
					{:else if members.length === 0}
						<p class="text-sm leading-normal text-muted-foreground">
							Nobody is on this team yet.
						</p>
					{:else}
						<ul class="flex flex-col rounded-lg border border-line-default" aria-live="polite">
							{#each members as member (member.accountId)}
								<li
									class="flex flex-wrap items-center gap-2 border-b border-line-subtle px-3 py-2 last:border-b-0"
								>
									<Avatar.Root size="sm">
										<Avatar.Fallback>{initialsOf(member.displayName)}</Avatar.Fallback>
									</Avatar.Root>
									<span class="min-w-0 flex-[1_1_120px] truncate text-md text-ink-900">
										{member.displayName}
									</span>
									<span class="min-w-0 truncate text-sm text-muted-foreground">{member.email}</span>
									<Button
										variant="ghost"
										size="icon-sm"
										disabled={locked}
										aria-label="Remove {member.displayName} from {team.name}"
										onclick={() => removeMember(member.accountId)}
									>
										<X aria-hidden="true" />
									</Button>
								</li>
							{/each}
						</ul>
					{/if}

					{#if !archived && !readOnly}
						<div class="flex flex-col gap-2" role="search">
							<label for="team-member-search" class="text-sm font-medium text-ink-900">
								Add someone
							</label>
							<Input
								id="team-member-search"
								type="search"
								enterkeyhint="search"
								autocapitalize="none"
								spellcheck="false"
								placeholder="Search {data.workspace.name} by name or email"
								disabled={busy}
								value={candidateQuery}
								oninput={(event) => searchCandidates(event.currentTarget.value)}
							/>

							{#if candidateQuery && candidates.length === 0 && !searching}
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									Nobody in {data.workspace.name} matches “{candidateQuery}”.
									<a
										href="/invite-teammates?workspace={slug}"
										class="text-link underline-offset-2 hover:text-link-hover hover:underline"
									>
										Invite them to {data.workspace.name}
									</a>
									first.
								</p>
							{:else if candidates.length > 0}
								<ul class="flex flex-col rounded-lg border border-line-default">
									{#each candidates as candidate (candidate.accountId)}
										<li class="border-b border-line-subtle last:border-b-0">
											<Button
												variant="ghost"
												class="h-auto w-full justify-start gap-2 rounded-none px-3 py-2"
												disabled={busy}
												onclick={() => addMember(candidate.accountId)}
											>
												<Avatar.Root size="sm">
													<Avatar.Fallback>{initialsOf(memberName(candidate))}</Avatar.Fallback>
												</Avatar.Root>
												<span class="min-w-0 flex-1 truncate text-left text-md text-ink-900">
													{memberName(candidate)}
												</span>
												<span class="shrink-0 text-sm text-muted-foreground">
													{adding === candidate.accountId ? "Adding" : "Add"}
												</span>
											</Button>
										</li>
									{/each}
								</ul>
							{/if}
						</div>
					{/if}
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
