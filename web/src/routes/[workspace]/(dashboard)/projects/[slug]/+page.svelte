<script lang="ts">
	import ActivityFeedView from "$lib/activity/activity-feed.svelte";
	import type { ActivityFeed } from "$lib/activity/activity";
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import EyeOff from "@lucide/svelte/icons/eye-off";
	import Target from "@lucide/svelte/icons/target";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import {
		groupByCategory,
		healthLabel,
		healths,
		projectFailureMessage,
		projectStates,
		projectsPath,
		readProjectFailure,
		stateLabel,
		type ProjectFailure,
		type ProjectHealth,
		type ProjectState,
	} from "$lib/projects/projects";
	import { categoryLabels } from "$lib/team/states";
	import { initialsOf } from "$lib/team/members";
	import { memberName, searchDebounceMs, type Membership } from "$lib/workspace/members";
	import { onCalendarDate, onDate, onDateAndTime } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import { projectPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV
			? projectPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const detail = $derived(preview?.detail ?? data.detail);
	const progress = $derived(preview?.progress ?? data.progress);

	const ready = $derived(detail.kind === "ready" ? detail : null);
	const project = $derived(ready?.project ?? null);
	const groups = $derived(groupByCategory(ready?.issues ?? []));
	const members = $derived(ready?.members ?? []);
	const latest = $derived(ready?.updates[0] ?? null);
	const earlier = $derived(ready?.updates.slice(1) ?? []);
	const posting = $derived(page.url.searchParams.has("status"));

	let health = $state<ProjectHealth>("on_track");
	let body = $state("");
	let working = $state(false);
	let failure = $state<ProjectFailure | null>(null);

	let candidateQuery = $state("");
	let candidates = $state<Membership[]>([]);
	let adding = $state("");
	let candidateDebounce: ReturnType<typeof setTimeout> | undefined;

	$effect(() => () => clearTimeout(candidateDebounce));

	function dismiss() {
		const next = new URL(page.url);
		next.searchParams.delete("status");
		goto(next, { replaceState: true, noScroll: true, keepFocus: true });
	}

	let loadedActivity = $state.raw<ActivityFeed | null>(null);

	const activity = $derived<ActivityFeed>(
		loadedActivity ?? (ready ? ready.activity : { kind: "loading" })
	);

	function when(instant: string): string {
		return onDateAndTime(instant, data.workspace.timezone);
	}

	async function moreActivity(): Promise<void> {
		const base = activity;

		if (!ready || base.kind !== "ready" || !base.nextCursor) return;

		working = true;

		try {
			const { data: page } = await api.GET(
				"/workspaces/{workspaceId}/projects/{projectId}/activity",
				{
					params: {
						path: { workspaceId: data.workspace.id, projectId: ready.project.id },
						query: { cursor: base.nextCursor },
					},
				}
			);

			if (page) {
				loadedActivity = {
					kind: "ready",
					events: [...base.events, ...page.events],
					nextCursor: page.nextCursor,
				};
			}
		} finally {
			working = false;
		}
	}

	async function act<T>(run: () => Promise<{ error?: unknown; data?: T }>) {
		working = true;
		failure = null;

		try {
			const { error } = await run();

			if (error) {
				failure = readProjectFailure(error);

				return false;
			}

			await invalidate(keys.projects(data.workspace.id));

			return true;
		} catch {
			failure = { kind: "unavailable" };

			return false;
		} finally {
			working = false;
		}
	}

	async function postStatus() {
		if (!project || !body.trim()) return;

		const done = await act(() =>
			api.POST("/workspaces/{workspaceId}/projects/{projectId}/status", {
				params: { path: { workspaceId: data.workspace.id, projectId: project.id } },
				body: { health, body: body.trim() },
			})
		);

		if (done) {
			body = "";
			dismiss();
		}
	}

	async function setState(next: ProjectState) {
		if (!project || project.state === next) return;

		await act(() =>
			api.PATCH("/workspaces/{workspaceId}/projects/{projectId}", {
				params: { path: { workspaceId: data.workspace.id, projectId: project.id } },
				body: { state: next },
			})
		);
	}

	async function setArchived(archive: boolean) {
		if (!project) return;

		await act(() =>
			api.POST(
				archive
					? "/workspaces/{workspaceId}/projects/{projectId}/archive"
					: "/workspaces/{workspaceId}/projects/{projectId}/unarchive",
				{ params: { path: { workspaceId: data.workspace.id, projectId: project.id } } }
			)
		);
	}

	async function remove() {
		if (!project) return;

		const done = await act(() =>
			api.DELETE("/workspaces/{workspaceId}/projects/{projectId}", {
				params: { path: { workspaceId: data.workspace.id, projectId: project.id } },
			})
		);

		if (done) await goto(projectsPath(slug));
	}

	async function findCandidates(query: string) {
		if (!project || !query) {
			candidates = [];

			return;
		}

		try {
			const { data: found } = await api.GET("/workspaces/{workspaceId}/members", {
				params: { path: { workspaceId: data.workspace.id }, query: { query, limit: 8 } },
			});

			candidates = (found?.members ?? []).filter(
				(candidate) =>
					!members.some((member) => member.accountId === candidate.accountId)
			);
		} catch {
			candidates = [];
		}
	}

	function searchCandidates(value: string) {
		candidateQuery = value;
		clearTimeout(candidateDebounce);
		candidateDebounce = setTimeout(() => findCandidates(value), searchDebounceMs);
	}

	async function addMember(accountId: string) {
		if (!project) return;

		adding = accountId;

		await act(() =>
			api.POST("/workspaces/{workspaceId}/projects/{projectId}/members", {
				params: { path: { workspaceId: data.workspace.id, projectId: project.id } },
				body: { accountId },
			})
		);

		candidates = candidates.filter((candidate) => candidate.accountId !== accountId);
		adding = "";
	}

	async function removeMember(accountId: string) {
		if (!project) return;

		await act(() =>
			api.DELETE("/workspaces/{workspaceId}/projects/{projectId}/members/{accountId}", {
				params: {
					path: { workspaceId: data.workspace.id, projectId: project.id, accountId },
				},
			})
		);
	}
</script>

<svelte:head>
	<title>{project ? project.name : "Project"} · {data.workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Target class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<a
				href={projectsPath(slug)}
				class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Projects
			</a>
			{#if project}
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
					{project.name}
				</h1>
				{#if progress}
					<ProgressBar {progress} class="ml-auto hidden lg:inline-flex" />
				{/if}
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if detail.kind === "loading"}
				<div class="h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if detail.kind === "not_found"}
				<div class="flex flex-col gap-2">
					<h2 class="text-md font-medium tracking-snug text-ink-900">No project here</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no project at this address in {data.workspace.name}.
					</p>
					<div>
						<Button variant="secondary" size="sm" href={projectsPath(slug)}>All projects</Button>
					</div>
				</div>
			{:else if detail.kind === "unavailable" || !project}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this project</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				{#if failure}
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>That did not go through</Alert.Title>
						<Alert.Description>{projectFailureMessage(failure)}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if project.archived}
					<Alert.Root variant="warning">
						<CircleX aria-hidden="true" />
						<Alert.Title>This project is archived</Alert.Title>
						<Alert.Description>
							It stays readable and its issues are untouched. Nothing about it can change until it
							is brought back.
						</Alert.Description>
						<Alert.Action>
							<Button
								variant="secondary"
								size="sm"
								disabled={working}
								onclick={() => setArchived(false)}
							>
								Bring it back
							</Button>
						</Alert.Action>
					</Alert.Root>
				{/if}

				<div class="flex flex-col gap-2">
					{#if project.description}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							{project.description}
						</p>
					{/if}
					<div class="flex flex-wrap items-center gap-2">
						{#if progress}
							<ProgressBar {progress} class="lg:hidden" />
						{/if}
						<span class="font-mono text-xs text-muted-foreground tabular-nums">
							{project.targetOn
								? `Target ${onCalendarDate(project.targetOn)}`
								: "No target date"}
						</span>
					</div>
				</div>

				{#if project.concealedWork}
					<Alert.Root>
						<EyeOff aria-hidden="true" />
						<Alert.Title>Some of this work is not yours to see</Alert.Title>
						<Alert.Description>
							This project also draws on a team you are not on, so the progress and the issues
							below cover your teams only.
						</Alert.Description>
					</Alert.Root>
				{/if}

				<section class="flex flex-col gap-2">
					<div class="flex items-baseline justify-between gap-2">
						<Eyebrow class="text-ink-600">Status</Eyebrow>
						{#if !project.archived && !posting}
							<Button variant="ghost" size="sm" href="{page.url.pathname}?status">
								Post an update
							</Button>
						{/if}
					</div>

					{#if posting}
						<div class="flex flex-col gap-3 rounded-lg border border-line-strong bg-paper-1 p-3">
							<div class="flex flex-col gap-1.5">
								<span class="text-sm text-ink-900" id="project-health-label">How is it going</span>
								<Select.Root
									type="single"
									value={health}
									onValueChange={(value) => (health = value as ProjectHealth)}
									disabled={working}
								>
									<Select.Trigger class="w-full" aria-labelledby="project-health-label">
										{healthLabel(health)}
									</Select.Trigger>
									<Select.Content>
										{#each healths as option (option)}
											<Select.Item value={option}>{healthLabel(option)}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>

							<div class="flex flex-col gap-1.5">
								<label for="project-status-body" class="text-sm text-ink-900">What changed</label>
								<Textarea
									id="project-status-body"
									rows={4}
									disabled={working}
									placeholder="Auth migration slipped a week; the rest is on track."
									bind:value={body}
								/>
							</div>

							<div class="flex gap-2">
								<Button disabled={working || !body.trim()} onclick={postStatus}>
									{working ? "Posting" : "Post update"}
								</Button>
								<Button variant="secondary" disabled={working} onclick={dismiss}>Cancel</Button>
							</div>
						</div>
					{/if}

					{#if latest}
						<div class="flex flex-col gap-1.5 rounded-lg border border-line-subtle p-3">
							<div class="flex items-center gap-2">
								<Tag
									name={healthLabel(latest.health)}
									color={latest.health === "on_track" ? "cyan" : "orchid"}
								/>
								<span class="text-sm text-muted-foreground">
									{latest.authorName || "Someone"} · {onDate(
										latest.createdAt,
										data.workspace.timezone
									)}
								</span>
							</div>
							<p class="text-md leading-normal text-ink-900 text-pretty">{latest.body}</p>
						</div>
					{:else if !posting}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Nobody has said how this is going yet. A percentage never says whether something is
							in trouble.
						</p>
					{/if}

					{#if earlier.length > 0}
						<ul class="flex flex-col gap-2">
							{#each earlier as update (update.id)}
								<li class="flex flex-col gap-1 border-l-2 border-line-subtle pl-3">
									<div class="flex items-center gap-2">
										<span class="text-xs text-muted-foreground">{healthLabel(update.health)}</span>
										<span class="text-xs text-muted-foreground">
											{update.authorName || "Someone"} · {onDate(
												update.createdAt,
												data.workspace.timezone
											)}
										</span>
									</div>
									<p class="text-sm leading-normal text-muted-foreground text-pretty">
										{update.body}
									</p>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="flex flex-col gap-2">
					<Eyebrow class="text-ink-600">Where it stands</Eyebrow>
					<div class="flex flex-col gap-1.5">
						<span class="text-sm text-ink-900" id="project-state-label">State</span>
						<Select.Root
							type="single"
							value={project.state}
							onValueChange={(value) => setState(value as ProjectState)}
							disabled={working || project.archived}
						>
							<Select.Trigger class="w-full" aria-labelledby="project-state-label">
								{stateLabel(project.state)}
							</Select.Trigger>
							<Select.Content>
								{#each projectStates as option (option)}
									<Select.Item value={option}>{stateLabel(option)}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<dl class="flex flex-wrap gap-x-8 gap-y-2 text-sm">
						<div class="flex items-baseline gap-2">
							<dt class="text-muted-foreground">Lead</dt>
							<dd class="text-ink-900">{project.leadName || "Nobody yet"}</dd>
						</div>
						<div class="flex items-baseline gap-2">
							<dt class="text-muted-foreground">Started</dt>
							<dd class="font-mono text-ink-900">
								{onDate(project.createdAt, data.workspace.timezone)}
							</dd>
						</div>
					</dl>
				</section>

				<section class="flex flex-col gap-2">
					<Eyebrow class="text-ink-600">Members</Eyebrow>
					{#if members.length === 0}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Nobody is on this project yet. Project members are separate from the teams the work
							comes from.
						</p>
					{:else}
						<ul class="flex flex-col rounded-lg border border-line-default">
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
										disabled={working || project.archived}
										aria-label="Take {member.displayName} off {project.name}"
										onclick={() => removeMember(member.accountId)}
									>
										<X aria-hidden="true" />
									</Button>
								</li>
							{/each}
						</ul>
					{/if}

					{#if !project.archived}
						<div class="flex flex-col gap-2" role="search">
							<label for="project-member-search" class="text-sm font-medium text-ink-900">
								Add someone
							</label>
							<Input
								id="project-member-search"
								type="search"
								enterkeyhint="search"
								autocapitalize="none"
								spellcheck="false"
								placeholder="Search {data.workspace.name} by name or email"
								disabled={working}
								value={candidateQuery}
								oninput={(event) => searchCandidates(event.currentTarget.value)}
							/>
							{#if candidates.length > 0}
								<ul class="flex flex-col rounded-lg border border-line-default">
									{#each candidates as candidate (candidate.accountId)}
										<li class="border-b border-line-subtle last:border-b-0">
											<Button
												variant="ghost"
												class="h-auto w-full justify-start gap-2 rounded-none px-3 py-2"
												disabled={working}
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

				<section class="flex flex-col gap-2">
					<Eyebrow class="text-ink-600">Issues</Eyebrow>
					{#if groups.length === 0}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							No issues are in this project yet. Put one in from the issue itself.
						</p>
					{:else}
						{#each groups as group (group.category)}
							<div class="flex flex-col">
								<div class="flex items-center gap-2 py-1.5">
									<StatusIcon category={group.category} decorative />
									<span class="text-sm text-ink-900">{categoryLabels[group.category]}</span>
									<span class="font-mono text-xs text-muted-foreground tabular-nums">
										{group.issues.length}
									</span>
								</div>
								<ul class="flex flex-col">
									{#each group.issues as issue (issue.id)}
										<li>
											<IssueRow
												{issue}
												href={workspacePath(slug, `/issues/${issue.reference}`)}
												now={data.now}
												timezone={data.workspace.timezone}
											/>
										</li>
									{/each}
								</ul>
							</div>
						{/each}
					{/if}
				</section>

				<section class="flex flex-col gap-3">
					<h2 class="text-md font-medium tracking-snug text-ink-900">History</h2>
					<ActivityFeedView
						feed={activity}
						{when}
						{working}
						emptyLine="Nothing has changed since this project was created."
						onmore={moreActivity}
					/>
				</section>

				<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							{project.archived ? "Delete this project" : "Archive or delete"}
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							{#if project.archived}
								Deleting {project.name} is permanent. Its issues are not deleted — they carry on
								without a project.
							{:else}
								Archiving takes {project.name} off the list without touching its issues; only a
								completed or cancelled project can be archived, and it can be brought back at any
								time. Deleting is permanent, and its issues carry on without a project.
							{/if}
						</p>
					</div>
					<div class="flex flex-wrap gap-2">
						{#if !project.archived}
							<Button variant="destructive" disabled={working} onclick={() => setArchived(true)}>
								Archive project
							</Button>
						{/if}
						<Button variant="ghost" disabled={working} onclick={remove}>
							{working ? "Deleting" : "Delete project"}
						</Button>
					</div>
				</section>
			{/if}
		</div>
	</div>
</div>
