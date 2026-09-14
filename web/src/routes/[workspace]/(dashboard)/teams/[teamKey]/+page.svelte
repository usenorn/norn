<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Lock from "@lucide/svelte/icons/lock";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Settings from "@lucide/svelte/icons/settings";
	import Users from "@lucide/svelte/icons/users";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import TeamMembers from "$lib/team/team-members.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { teamIssuesPath } from "$lib/issues/listing";
	import { teamProjectsPath } from "$lib/projects/projects";
	import { workspacePath } from "$lib/workspace/navigation";
	import { teamPath, teamSettingsPath } from "$lib/team/teams";
	import { teamProfileSchema } from "$lib/team/team-profile-schema";
	import { managesTeams } from "$lib/workspace/members";
	import { teamOverviewPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV
			? teamOverviewPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const overview = $derived(preview?.overview ?? data.overview);
	const team = $derived(overview.kind === "ready" ? overview.team : null);
	const here = $derived(team ? teamPath(slug, team.key) : "");
	const readOnly = $derived(!managesTeams(data.members, data.member.id));

	let editing = $state(false);
	let failure = $state<string | null>(null);

	const form = superForm(defaults(zod4(teamProfileSchema)), {
		id: "team-profile",
		SPA: true,
		validators: zod4Client(teamProfileSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid || !team) return;

			failure = null;

			const { error } = await api.PATCH("/workspaces/{workspaceId}/teams/{teamId}", {
				params: { path: { workspaceId: data.workspace.id, teamId: team.id } },
				body: { name: pending.data.name, description: pending.data.description },
			});

			if (error) {
				failure =
					error.status === 403
						? "Only workspace admins can change a team."
						: "Nothing changed. Wait a moment and try again.";

				return;
			}

			editing = false;
			await invalidate(keys.page(page.route.id));
		},
	});

	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		if (!team) return;

		const { name, description } = team;

		formData.update((current) => ({ ...current, name, description }), { taint: false });
	});

	function edit() {
		failure = null;
		editing = true;
	}

	function cancel() {
		editing = false;
		failure = null;

		if (!team) return;

		const { name, description } = team;

		formData.update((current) => ({ ...current, name, description }), { taint: false });
	}
</script>

<svelte:head>
	<title>{team ? team.name : "Team"} · {data.workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			{#if team?.visibility === "private"}
				<Lock class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			{:else}
				<Users class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			{/if}
			<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
				{team ? team.name : "Team"}
			</h1>
			{#if team}
				<TeamKey key={team.key} />
				<div class="min-w-2 flex-1"></div>
				<Button variant="outline" size="sm" href={teamSettingsPath(slug, team.key)}>
					<Settings aria-hidden="true" />
					Team settings
				</Button>
			{/if}
		</div>

		{#if team}
			<nav class="flex items-center gap-1 px-3 pb-1" aria-label="Team">
				<a
					href={here}
					aria-current={data.tab === "overview" ? "page" : undefined}
					class="rounded-sm px-2 py-1 text-sm text-muted-foreground motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring aria-[current=page]:bg-accent aria-[current=page]:text-ink-900"
				>
					Overview
				</a>
				<a
					href="{here}?tab=members"
					aria-current={data.tab === "members" ? "page" : undefined}
					class="rounded-sm px-2 py-1 text-sm text-muted-foreground motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring aria-[current=page]:bg-accent aria-[current=page]:text-ink-900"
				>
					Members
				</a>
			</nav>
		{/if}
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if overview.kind === "loading"}
				<div class="h-24 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if overview.kind === "not_found"}
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
			{:else if overview.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this team</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else if data.tab === "members"}
				<section class="flex flex-col gap-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Members</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Belonging to {data.workspace.name} does not put somebody on this team.
						</p>
					</div>

					<TeamMembers
						workspace={{ id: data.workspace.id, name: data.workspace.name, slug }}
						team={{ id: overview.team.id, name: overview.team.name }}
						roster={preview?.roster ?? data.roster}
						archived={overview.team.status === "archived"}
						{readOnly}
					/>
				</section>
			{:else}
				<section class="flex flex-col gap-4">
					{#if failure}
						<Alert.Root variant="destructive">
							<CircleX aria-hidden="true" />
							<Alert.Title>That did not work</Alert.Title>
							<Alert.Description>{failure}</Alert.Description>
						</Alert.Root>
					{/if}

					{#if editing}
						<form method="POST" use:enhance class="flex flex-col gap-4">
							<Form.Field {form} name="name">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Team name</Form.Label>
										<Input {...props} disabled={$submitting} bind:value={$formData.name} />
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
											rows={3}
											disabled={$submitting}
											placeholder="What this team looks after."
											bind:value={$formData.description}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<div class="flex flex-wrap gap-2">
								<Button type="submit" size="sm" disabled={$submitting}>
									{$submitting ? "Saving" : "Save"}
								</Button>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									disabled={$submitting}
									onclick={cancel}
								>
									Cancel
								</Button>
							</div>
						</form>
					{:else}
						<div class="flex items-start gap-2">
							<div class="flex min-w-0 flex-1 flex-col gap-1">
								<h2 class="text-lg font-medium tracking-title text-ink-900">
									{overview.team.name}
								</h2>
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									{overview.team.description || "No description yet."}
								</p>
							</div>
							{#if !readOnly}
								<Button variant="ghost" size="sm" onclick={edit}>
									<Pencil aria-hidden="true" />
									Edit
								</Button>
							{/if}
						</div>
					{/if}

					<div class="flex flex-wrap gap-2">
						<Button variant="secondary" size="sm" href={teamIssuesPath(slug, overview.team.key)}>
							Issues
						</Button>
						<Button variant="secondary" size="sm" href={teamProjectsPath(slug, overview.team.key)}>
							Projects
						</Button>
					</div>
				</section>
			{/if}
		</div>
	</div>
</div>
