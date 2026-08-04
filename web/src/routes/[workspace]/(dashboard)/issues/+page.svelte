<script lang="ts">
	import { invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import Bell from "@lucide/svelte/icons/bell";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Kanban from "@lucide/svelte/icons/kanban";
	import List from "@lucide/svelte/icons/list";
	import Plus from "@lucide/svelte/icons/plus";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import {
		boardFor,
		countForTab,
		issueTabs,
		tabLabels,
		type Issue,
	} from "$lib/issues/board";
	import { workspacePath } from "$lib/workspace/navigation";
	import { issuesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? issuesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	const slug = $derived(data.workspace.slug);
	const at = $derived((path: string) => workspacePath(slug, path));

	let moved = $state<Record<string, Issue["state"]>>({});
	let dragging = $state<string | null>(null);
	let dropTarget = $state<string | null>(null);
	let failed = $state(false);
	let creating = $state(false);
	let title = $state("");

	const team = $derived(preview?.team ?? data.team);
	const states = $derived(preview?.states ?? data.states);
	const progress = $derived(preview?.progress ?? data.progress);

	const issues = $derived(
		(preview?.issues ?? data.issues)?.map((issue) =>
			moved[issue.id] ? { ...issue, state: moved[issue.id] } : issue
		)
	);

	const board = $derived(
		boardFor(issues, states, data.tab, team?.name ?? "", { showEmpty: data.showEmpty })
	);
	const groups = $derived(board.kind === "ready" ? board.groups : []);
	const assigneeNames = $derived(
		new Map((data.members ?? []).map((member) => [member.accountId, member.displayName ?? ""]))
	);
	const flat = $derived(groups.flatMap((group) => group.issues));

	const params = $derived.by(() => {
		const q = new URLSearchParams();
		if (team) q.set("team", team.key);
		if (data.tab !== "open") q.set("tab", data.tab);
		if (data.layout !== "list") q.set("layout", data.layout);
		if (data.showEmpty) q.set("empty", "1");

		return q;
	});

	const linkWith = $derived((changes: Record<string, string | null>) => {
		const q = new URLSearchParams(params);

		for (const [key, value] of Object.entries(changes)) {
			if (value === null) q.delete(key);
			else q.set(key, value);
		}

		const query = q.toString();

		return at(`/issues${query ? `?${query}` : ""}`);
	});

	async function moveIssue(issue: Issue, stateId: string) {
		const target = (states ?? []).find((state) => state.id === stateId);
		if (!target || issue.state.id === stateId) return;

		const before = issue.state;
		failed = false;
		moved = {
			...moved,
			[issue.id]: {
				id: target.id,
				name: target.name,
				category: target.category,
				position: target.position,
			},
		};

		try {
			const { error } = await api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { expectedVersion: issue.version, stateId },
			});

			if (error) {
				moved = { ...moved, [issue.id]: before };
				failed = true;

				return;
			}

			await invalidateAll();
		} catch {
			moved = { ...moved, [issue.id]: before };
			failed = true;
		}
	}

	async function createIssue(event: SubmitEvent) {
		event.preventDefault();

		if (!team || !title.trim()) return;

		creating = true;
		failed = false;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/issues", {
				params: { path: { workspaceId: data.workspace.id } },
				body: { teamId: team.id, title: title.trim() },
			});

			if (error) {
				failed = true;

				return;
			}

			title = "";
			await invalidateAll();
		} catch {
			failed = true;
		} finally {
			creating = false;
		}
	}

	function onDragStart(event: DragEvent, id: string) {
		dragging = id;
		event.dataTransfer?.setData("text/plain", id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
	}

	function onDragEnd() {
		dragging = null;
		dropTarget = null;
	}

	function onDragOver(event: DragEvent, stateId: string) {
		if (!dragging) return;
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
		dropTarget = stateId;
	}

	function onDrop(event: DragEvent, stateId: string) {
		event.preventDefault();

		const id = event.dataTransfer?.getData("text/plain") || dragging;
		onDragEnd();

		const issue = (issues ?? []).find((candidate) => candidate.id === id);
		if (issue) moveIssue(issue, stateId);
	}

	let cursor = $state(0);

	function onkeydown(event: KeyboardEvent) {
		if (event.metaKey || event.ctrlKey || event.altKey) return;

		const target = event.target as HTMLElement | null;

		if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)))
			return;

		if (event.key === "j" || event.key === "ArrowDown") {
			event.preventDefault();
			cursor = Math.min(cursor + 1, flat.length - 1);
		} else if (event.key === "k" || event.key === "ArrowUp") {
			event.preventDefault();
			cursor = Math.max(cursor - 1, 0);
		}
	}
</script>

<svelte:head><title>Issues · Norn</title></svelte:head>
<svelte:window {onkeydown} />

<div class="relative flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
				<List class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Issues</h1>

				{#if data.teams.length > 1 && team}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props })}
								<Button {...props} variant="ghost" size="sm" class="shrink-0">
									<TeamKey key={team.key} />
									{team.name}
									<ChevronDown aria-hidden="true" />
								</Button>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="start">
							<DropdownMenu.Label>Team</DropdownMenu.Label>
							{#each data.teams as candidate (candidate.id)}
								<DropdownMenu.Item>
									{#snippet child({ props })}
										<a href={linkWith({ team: candidate.key })} {...props}>
											<span class="flex-1">{candidate.name}</span>
											{#if candidate.id === team.id}
												<span class="font-mono text-2xs text-ink-600">✓</span>
											{/if}
										</a>
									{/snippet}
								</DropdownMenu.Item>
							{/each}
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{:else if team}
					<span class="shrink-0 text-md text-muted-foreground">{team.name}</span>
				{/if}

				{#if issues}
					<div role="tablist" class="ml-1 hidden shrink-0 items-center gap-3 sm:flex">
						{#each issueTabs as tab (tab)}
							<a
								href={linkWith({ tab: tab === "open" ? null : tab })}
								role="tab"
								aria-selected={data.tab === tab}
								data-active={data.tab === tab}
								class="relative inline-flex h-7.5 items-center gap-1.5 text-md font-medium whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-transparent after:transition-colors after:duration-110 after:ease-out hover:text-ink-900 hover:after:bg-line-strong data-[active=true]:text-ink-900 data-[active=true]:after:bg-primary"
							>
								{tabLabels[tab]}
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{countForTab(issues, tab)}
								</span>
							</a>
						{/each}
					</div>
				{/if}

				{#if progress}
					<ProgressBar {progress} class="hidden lg:inline-flex" />
				{/if}
			</div>

			<div class="flex flex-none gap-1">
				<Button
					href={linkWith({ layout: null })}
					variant="outline"
					size="icon-sm"
					aria-label="List view"
					aria-pressed={data.layout === "list"}
					class={data.layout === "list" ? "border-primary bg-primary text-primary-foreground" : ""}
				>
					<List class="size-icon-toolbar" aria-hidden="true" />
				</Button>
				<Button
					href={linkWith({ layout: "board" })}
					variant="outline"
					size="icon-sm"
					aria-label="Board view"
					aria-pressed={data.layout === "board"}
					class={data.layout === "board" ? "border-primary bg-primary text-primary-foreground" : ""}
				>
					<Kanban class="size-icon-toolbar" aria-hidden="true" />
				</Button>
			</div>
			<Button variant="outline" size="icon-sm" aria-label="Notifications">
				<Bell class="size-icon-toolbar" aria-hidden="true" />
			</Button>
		</div>

		{#if team}
			<form
				onsubmit={createIssue}
				class="flex h-11 items-center gap-2 border-t border-line-subtle pr-3 pl-3.5"
			>
				<label for="new-issue-title" class="sr-only">New issue in {team.name}</label>
				<Input
					id="new-issue-title"
					bind:value={title}
					disabled={creating}
					placeholder="What needs doing in {team.name}?"
					class="h-7.5 flex-1"
				/>
				<Button type="submit" size="sm" disabled={creating || !title.trim()}>
					<Plus aria-hidden="true" />
					{creating ? "Adding" : "New issue"}
				</Button>
			</form>
		{/if}

		<div class="flex h-8.5 items-center gap-3 overflow-hidden border-t border-line-subtle pr-3 pl-3.5">
			<span class="text-sm whitespace-nowrap text-muted-foreground">
				Grouped by {team ? `${team.name} states` : "state"}
			</span>
			<div class="min-w-2 flex-1"></div>
			<a
				href={linkWith({ empty: data.showEmpty ? null : "1" })}
				class="text-sm whitespace-nowrap text-muted-foreground hover:text-foreground"
			>
				{data.showEmpty ? "Hide empty states" : "Show empty states"}
			</a>
			{#if team}
				<a
					href={at(`/settings/teams/${team.key.toLowerCase()}`)}
					class="text-sm whitespace-nowrap text-link underline-offset-2 hover:text-link-hover hover:underline"
				>
					Edit states
				</a>
			{/if}
		</div>
	</div>

	<div class="relative flex min-h-0 flex-1 flex-col">
		{#if failed}
			<div class="px-4 pt-3">
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not stick</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			</div>
		{/if}

		{#if board.kind === "unavailable"}
			<div class="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">We could not load these issues</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Nothing changed. Wait a moment and try again.
				</p>
			</div>
		{:else if board.kind === "no_teams" || !team}
			<div class="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">No teams yet</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Issues belong to a team. Make one first, and it arrives with a set of states ready to use.
				</p>
				<Button href={at("/settings/teams")} variant="secondary" size="sm">Create a team</Button>
			</div>
		{:else if board.kind === "empty"}
			<div class="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">
					Nothing {data.tab === "all" ? "" : `${tabLabels[data.tab].toLowerCase()} `}in {board.team}
				</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Raise the first one above, and it starts in whichever state this team files new work into.
				</p>
			</div>
		{:else if data.layout === "list"}
			<div class="flex-1 overflow-auto">
				{#each groups as group (group.state.id)}
					{@const offset = flat.indexOf(group.issues[0])}
					<section
						role="group"
						aria-label={group.state.name}
						ondragover={(event) => onDragOver(event, group.state.id)}
						ondrop={(event) => onDrop(event, group.state.id)}
						data-dropping={dropTarget === group.state.id}
						class="data-[dropping=true]:bg-accent"
					>
						<div
							class="sticky top-0 z-1 flex h-7.5 items-center gap-2 border-b border-line-default bg-background pr-3 pl-1.5"
						>
							<span class="flex h-6 items-center gap-1.5 rounded-sm px-1">
								<StatusIcon category={group.state.category} decorative />
								<span class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
									{group.state.name}
								</span>
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{group.issues.length}
								</span>
							</span>
							<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
						</div>
						{#each group.issues as issue, index (issue.id)}
							<IssueRow
								{issue}
								href={at(`/issues/${issue.reference}`)}
								assignee={assigneeNames.get(issue.assigneeAccountId ?? "") ?? ""}
								now={data.now}
								timezone={data.workspace.timezone}
								cursor={offset + index === cursor}
								draggable
								dragging={dragging === issue.id}
								ondragstart={(event) => onDragStart(event, issue.id)}
								ondragend={onDragEnd}
							/>
						{/each}
					</section>
				{/each}
			</div>
		{:else}
			<div class="flex-1 overflow-auto bg-background p-4">
				<div class="flex min-h-full items-start gap-3">
					{#each groups as group (group.state.id)}
						<div
							role="group"
							aria-label={group.state.name}
							ondragover={(event) => onDragOver(event, group.state.id)}
							ondrop={(event) => onDrop(event, group.state.id)}
							data-dropping={dropTarget === group.state.id}
							class="flex w-62.5 flex-none flex-col gap-2 rounded-lg border border-transparent p-1 transition-colors duration-70 ease-out data-[dropping=true]:border-dashed data-[dropping=true]:border-ink-400 data-[dropping=true]:bg-accent"
						>
							<div class="flex h-7 items-center gap-2 px-1">
								<StatusIcon category={group.state.category} decorative />
								<span class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
									{group.state.name}
								</span>
								<span class="font-mono text-2xs text-muted-foreground tabular-nums">
									{group.issues.length}
								</span>
								<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
							</div>
							{#each group.issues as issue (issue.id)}
								<a
									href={at(`/issues/${issue.reference}`)}
									draggable="true"
									ondragstart={(event) => onDragStart(event, issue.id)}
									ondragend={onDragEnd}
									data-dragging={dragging === issue.id}
									class="flex cursor-grab flex-col gap-2 rounded-lg border border-line-default bg-card p-2.5 transition-colors duration-70 ease-out hover:border-ink-400 active:cursor-grabbing data-[dragging=true]:opacity-40"
								>
									<span class="font-mono text-xs text-muted-foreground">{issue.reference}</span>
									<span class="text-md leading-snug tracking-snug text-ink-900">{issue.title}</span>
									{#if issue.labels.length > 0}
										<div class="flex flex-wrap items-center gap-2">
											{#each issue.labels as label (label.id)}
												<Tag name={label.name} color={label.color} />
											{/each}
										</div>
									{/if}
								</a>
							{/each}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<div
		class="hidden h-7.5 flex-none items-center justify-end gap-4 border-t border-line-subtle bg-card px-3.5 md:flex"
	>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="⌘ K" />go anywhere
		</span>
		<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
			<Kbd keys="↑ ↓" />move
		</span>
	</div>
</div>
