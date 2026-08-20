<script lang="ts">
	import { page } from "$app/state";
	import Bell from "@lucide/svelte/icons/bell";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import Funnel from "@lucide/svelte/icons/funnel";
	import Plus from "@lucide/svelte/icons/plus";
	import Settings from "@lucide/svelte/icons/settings";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import { bindShortcuts } from "$lib/shortcuts/registry.svelte";
	import { nthState, setStatus, statusIndexOf, statusMessage } from "$lib/issues/set-status";
	import { useToasts } from "$lib/toast/toasts.svelte";
	import TaskRow from "$lib/components/norn/task-row.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { workspacePath } from "$lib/workspace/navigation";
	import { bucketsOf } from "$lib/tasks/tasks";
	import type { TaskBucket } from "$lib/tasks/types";
	import { api } from "$lib/api";
	import type { Issue } from "$lib/issues/issues";
	import { myTasksPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? myTasksPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	type Loaded = { source: Issue[]; issues: Issue[]; nextCursor: string | undefined };
	type Paging = { kind: "idle" } | { kind: "loading" } | { kind: "unavailable" };

	let accumulated = $state.raw<Loaded | null>(null);
	let localPaging = $state<Paging>({ kind: "idle" });

	const loaded = $derived(
		accumulated && accumulated.source === data.issues ? accumulated.issues : data.issues
	);
	const nextCursor = $derived(
		accumulated && accumulated.source === data.issues ? accumulated.nextCursor : data.nextCursor
	);
	const paging = $derived<Paging>(preview?.paging ?? localPaging);

	const buckets = $derived<TaskBucket[]>(
		preview?.buckets ?? bucketsOf(loaded, data.assignee, data.now, data.workspace.timezone)
	);
	const total = $derived(buckets.reduce((sum, bucket) => sum + bucket.tasks.length, 0));

	async function loadMore() {
		if (!nextCursor) return;

		const source = data.issues;
		localPaging = { kind: "loading" };

		try {
			const { data: next, error } = await api.POST("/workspaces/{workspaceId}/issues/query", {
				params: { path: { workspaceId: data.workspace.id } },
				body: { ...data.query, cursor: nextCursor },
			});

			if (error || !next) {
				localPaging = { kind: "unavailable" };

				return;
			}

			accumulated = {
				source,
				issues: [...loaded, ...next.issues],
				nextCursor: next.nextCursor,
			};
			localPaging = { kind: "idle" };
		} catch {
			localPaging = { kind: "unavailable" };
		}
	}

	const flat = $derived(buckets.flatMap((bucket) => bucket.tasks));

	const cursor = listCursor(() => ({
		rows: flat,
		open: (task) => void goto(workspacePath(data.workspace.slug, `/issues/${task.id}`)),
	}));

	const issueOf = $derived(new Map(loaded.map((issue) => [issue.reference, issue])));

	const toasts = useToasts();

	bindShortcuts({
		"status-set": (binding) => void moveStatus(statusIndexOf(binding)),
	});

	async function moveStatus(nth: number) {
		const task = cursor.row;
		const issue = task && issueOf.get(task.id);
		const state = issue && nthState(data.states, issue.teamId, nth);

		if (!issue || !state) return;

		const outcome = await setStatus(data.workspace.id, issue, state);

		if (outcome.kind !== "unchanged") {
			toasts.show(statusMessage(outcome, issue.reference), {
				href: workspacePath(data.workspace.slug, `/issues/${issue.reference}`),
			});
		}

		if (outcome.kind === "changed") await invalidate(keys.issues(data.workspace.id));
	}
</script>

<svelte:head><title>My tasks · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<div class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
				<CircleDot class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">My tasks</h1>
				<span class="font-mono text-xs text-muted-foreground tabular-nums">{total}</span>
			</div>
			<Button variant="outline" size="icon-sm" aria-label="Notifications">
				<Bell class="size-icon-toolbar" aria-hidden="true" />
			</Button>
			<Button size="sm">
				<Plus aria-hidden="true" />
				New task
			</Button>
		</div>
		<div class="flex h-8.5 items-center gap-2.5 overflow-hidden border-t border-line-subtle pr-3 pl-3.5">
			<Button variant="ghost" size="sm">
				<Funnel aria-hidden="true" />
				Filter
			</Button>
			<div class="min-w-2 flex-1"></div>
			<span class="text-sm whitespace-nowrap text-muted-foreground">Grouped by due date</span>
			<Button variant="ghost" size="icon-sm" aria-label="Display options">
				<Settings class="size-icon-toolbar" aria-hidden="true" />
			</Button>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		{#if buckets.length === 0}
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
						/>
					{/each}
				</section>
			{/each}
		{/if}

		{#if nextCursor || paging.kind === "unavailable"}
			<div class="flex flex-col items-center gap-2 border-t border-line-subtle px-4 py-4">
				{#if paging.kind === "unavailable"}
					<p role="status" class="text-sm text-muted-foreground">
						We could not load any more. Nothing changed &mdash; try again.
					</p>
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

	<ShortcutBar ids={["cursor-down", "cursor-open", "status-set", "issue-new", "help"]} />
</div>
