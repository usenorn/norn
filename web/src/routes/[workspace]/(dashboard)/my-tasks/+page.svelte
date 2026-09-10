<script lang="ts">
	import { page } from "$app/state";
	import Bell from "@lucide/svelte/icons/bell";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import Plus from "@lucide/svelte/icons/plus";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import { bindShortcuts } from "$lib/shortcuts/registry.svelte";
	import { nthState, setStatus, statusIndexOf, statusMessage } from "$lib/issues/set-status";
	import { showFailure, showToast } from "$lib/toast/toasts";
	import TaskRow from "$lib/components/norn/task-row.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { attempt } from "$lib/api/attempt";
	import { cursorOf, grew, moreFailedLine, rowsOf, type Listed } from "$lib/api/listed";
	import { workspacePath } from "$lib/workspace/navigation";
	import { bucketsOf, groupsOf } from "$lib/tasks/tasks";
	import type { TaskBucket } from "$lib/tasks/types";
	import { api } from "$lib/api";
	import DisplayMenu from "$lib/issues/display-menu.svelte";
	import FilterBar from "$lib/issues/filter-bar.svelte";
	import { clearedLink, dueEntries, type FacetCatalogue } from "$lib/issues/facet-options";
	import { priorities } from "$lib/issues/issues";
	import { linkTo } from "$lib/issues/linking";
	import { surfaceDefaults, surfaceGroupings, surfaceOrderings, writeDisplay } from "$lib/issues/display";
	import { pickableFacets, type FacetKind } from "$lib/issues/facets";
	import { registerNewIssue, useNewIssue } from "$lib/issues/new-issue.svelte";
	import type { ColumnPaging } from "$lib/issues/paging";
	import type { Issue } from "$lib/issues/issues";
	import { myTasksPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? myTasksPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let accumulated = $state.raw<{ source: Listed<Issue>; rows: Listed<Issue> } | null>(null);
	let localPaging = $state<ColumnPaging>({ kind: "idle" });

	const rows = $derived<Listed<Issue>>(
		preview?.rows ??
			(accumulated && accumulated.source === data.rows ? accumulated.rows : data.rows)
	);
	const loaded = $derived(rowsOf(rows));
	const nextCursor = $derived(cursorOf(rows));
	const paging = $derived<ColumnPaging>(preview?.paging ?? localPaging);

	const raising = useNewIssue();

	registerNewIssue(() => ({ seed: { assigneeId: data.assigneeId } }));

	const basePath = $derived(workspacePath(data.workspace.slug, "/my-tasks"));

	const facets = $derived(preview ? {} : data.facets);

	const params = $derived.by(() => {
		const q = new URLSearchParams();

		for (const [kind, value] of Object.entries(facets)) if (value) q.set(kind, value);

		for (const [key, value] of writeDisplay(data.display)) q.set(key, value);

		return q;
	});

	const linkWith = $derived((changes: Record<string, string | null>) =>
		linkTo(basePath, params, changes)
	);

	const offered = $derived<FacetKind[]>(pickableFacets.filter((kind) => kind !== "assignee"));

	const catalogue = $derived<FacetCatalogue>({
		state: data.states.map((state) => ({ value: state.id, label: state.name })),
		priority: priorities.map((entry) => ({ value: entry.value, label: entry.label })),
		label: (data.labels ?? []).map((label) => ({ value: label.id, label: label.name })),
		project: (data.projects ?? []).map((project) => ({ value: project.id, label: project.name })),
		due: dueEntries(),
	});

	const cleared = $derived(clearedLink(offered, linkWith));

	const buckets = $derived<TaskBucket[]>(
		preview?.buckets ??
			(data.display.grouping === "due"
				? bucketsOf(loaded, data.assignee, data.now, data.workspace.timezone)
				: groupsOf(loaded, data.assignee, data.display.grouping, {
						states: data.states,
						projects: data.projects ?? [],
					}))
	);
	const total = $derived(buckets.reduce((sum, bucket) => sum + bucket.tasks.length, 0));

	async function loadMore() {
		if (!nextCursor) return;

		const source = data.rows;

		localPaging = { kind: "loading" };

		const outcome = await attempt({
			run: () =>
				api.POST("/workspaces/{workspaceId}/issues/query", {
					params: { path: { workspaceId: data.workspace.id } },
					body: { ...data.query, cursor: nextCursor },
				}),
		});

		if (outcome.kind !== "done") {
			localPaging = { kind: "unavailable" };

			return;
		}

		accumulated = {
			source,
			rows: grew(rows, {
				rows: outcome.value.issues,
				nextCursor: outcome.value.nextCursor,
			}),
		};
		localPaging = { kind: "idle" };
	}

	const flat = $derived(buckets.flatMap((bucket) => bucket.tasks));

	const cursor = listCursor(() => ({
		rows: flat,
		open: (task) => void goto(workspacePath(data.workspace.slug, `/issues/${task.id}`)),
	}));

	const issueOf = $derived(new Map(loaded.map((issue) => [issue.reference, issue])));

	bindShortcuts({
		"status-set": (binding) => void moveStatus(statusIndexOf(binding)),
	});

	async function moveStatus(nth: number) {
		const task = cursor.row;
		const issue = task && issueOf.get(task.id);
		const state = issue && nthState(data.states, issue.teamId, nth);

		if (!issue || !state) return;

		const outcome = await setStatus(data.workspace.id, issue, state);
		const href = workspacePath(data.workspace.slug, `/issues/${issue.reference}`);

		if (outcome.kind === "changed") {
			showToast(statusMessage(outcome, issue.reference), { href });
			await invalidate(keys.issues(data.workspace.id));

			return;
		}

		if (outcome.kind !== "unchanged") showFailure(statusMessage(outcome, issue.reference), { href });
	}
</script>

<svelte:head><title>My tasks · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
				<CircleDot class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">My tasks</h1>
				{#if rows.kind === "ready"}
					<span class="font-mono text-xs text-muted-foreground tabular-nums">
						{total}{nextCursor ? "+" : ""}
					</span>
				{/if}
			</div>
			<Button
				href={workspacePath(data.workspace.slug, "/inbox")}
				variant="outline"
				size="icon-sm"
				aria-label="Notifications"
			>
				<Bell class="size-icon-toolbar" aria-hidden="true" />
			</Button>
			<Button size="sm" onclick={() => raising.raise()}>
				<Plus aria-hidden="true" />
				New task
			</Button>
		</div>
		<div
			class="flex min-h-8.5 flex-wrap items-center gap-x-3 gap-y-1 border-t border-line-subtle py-1 pr-3 pl-3.5"
		>
			<FilterBar {facets} {offered} {catalogue} {linkWith} />

			<div class="min-w-2 flex-1"></div>

			<div class="flex shrink-0 items-center gap-1">
				<DisplayMenu
					display={data.display}
					defaults={surfaceDefaults.tasks}
					groupings={surfaceGroupings.tasks}
					orderings={surfaceOrderings.tasks}
					{linkWith}
					emptyGroups={false}
				/>
			</div>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		{#if rows.kind === "loading"}
			<div class="flex flex-col gap-1.5 p-3.5" aria-busy="true">
				{#each [0, 1, 2] as row (row)}
					<div class="h-7.5 animate-breathe rounded-md bg-paper-2"></div>
				{/each}
			</div>
		{:else if rows.kind === "unavailable"}
			<div class="flex flex-col items-center gap-3 px-6 py-12 text-center">
				<p class="text-md font-medium tracking-snug text-ink-900">
					We could not read your tasks
				</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Nothing has changed. This is a problem reaching Norn, not an empty workload.
				</p>
				<Button
					variant="secondary"
					size="sm"
					onclick={() => void invalidate(keys.page(page.route.id))}
				>
					Try again
				</Button>
			</div>
		{:else if rows.kind === "no_matches"}
			<div class="flex flex-col items-center gap-3 px-6 py-12 text-center">
				<p class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase">
					No tasks match these filters
				</p>
				<Button href={cleared} variant="secondary" size="sm">Clear filters</Button>
			</div>
		{:else if rows.kind === "empty"}
			<div class="flex flex-col items-center gap-1.5 px-6 py-12 text-center">
				<span
					class="mb-1.5 flex size-8 items-center justify-center border-b-2 border-line-strong text-ink-300"
				>
					<CircleDot class="size-icon-toolbar" aria-hidden="true" />
				</span>
				<p class="text-md font-medium tracking-snug text-ink-900">Nothing assigned to you</p>
				<p class="max-w-75 text-md leading-normal text-muted-foreground">
					Work assigned to you lands here, grouped by when it is due.
				</p>
			</div>
		{:else}
			{#each buckets as bucket (bucket.key)}
				<section>
					<div
						class="sticky top-0 z-1 flex h-7.5 items-center gap-2 border-b border-line-default bg-background pr-3 pl-3.5"
					>
						<span
							class="font-mono text-2xs font-medium tracking-eyebrow uppercase {bucket.emphasis
								? 'text-ink-900'
								: 'text-ink-600'}"
						>
							{bucket.label}
						</span>
						<span class="font-mono text-2xs text-muted-foreground tabular-nums">
							{bucket.tasks.length}
						</span>
						<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
					</div>
					{#each bucket.tasks as task (task.id)}
						<TaskRow
							{task}
							href={workspacePath(data.workspace.slug, `/issues/${task.id}`)}
							cursor={cursor.holds(task)}
							shown={data.display.shown}
						/>
					{/each}
				</section>
			{/each}
		{/if}

		{#if nextCursor || paging.kind === "unavailable"}
			<div class="flex flex-col items-center gap-2 border-t border-line-subtle px-4 py-4">
				{#if paging.kind === "unavailable"}
					<p role="status" class="text-sm text-muted-foreground">{moreFailedLine}</p>
				{/if}
				{#if nextCursor}
					<Button
						variant="secondary"
						size="sm"
						onclick={loadMore}
						disabled={paging.kind === "loading"}
					>
						{paging.kind === "loading" ? "Loading" : "Load more"}
					</Button>
				{/if}
			</div>
		{/if}
	</div>

	<ShortcutBar ids={["cursor-down", "cursor-open", "status-set", "issue-new", "issue-filter", "help"]} />
</div>
