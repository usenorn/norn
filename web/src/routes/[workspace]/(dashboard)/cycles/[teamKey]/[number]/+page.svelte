<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Layers from "@lucide/svelte/icons/layers";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import { api } from "$lib/api";
	import {
		cycleFailureMessage,
		openIssues,
		phaseLabel,
		readCycleFailure,
		scopeChangesOf,
		teamCyclesPath,
		type CycleFailure,
		type CycleRollover,
	} from "$lib/cycles/cycles";
	import { groupByState } from "$lib/issues/board";
	import { cycleWindow, onCalendarDate } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import { cyclePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? cyclePreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const detail = $derived(preview?.detail ?? data.detail);
	const progress = $derived(preview?.progress ?? data.progress);
	const states = $derived(preview?.states ?? data.states);

	const scope = $derived(detail.kind === "ready" ? detail.scope : undefined);
	const inScope = $derived(scope ? [...scope.original, ...scope.added] : []);
	const addedIds = $derived(new Set((scope?.added ?? []).map((issue) => issue.id)));
	const joined = $derived(scope ? scopeChangesOf(scope, "added") : []);
	const removed = $derived(scope ? scopeChangesOf(scope, "removed") : []);
	const movedOn = $derived(
		scope ? [...scopeChangesOf(scope, "rolled_over"), ...scopeChangesOf(scope, "returned")] : []
	);
	const groups = $derived(states ? groupByState(inScope, states, undefined) : []);
	const unfinished = $derived(openIssues(inScope));

	const closing = $derived(page.url.searchParams.has("close"));
	const canClose = $derived(detail.kind === "ready" && detail.cycle.phase === "ended");

	let rollover = $state<CycleRollover>("next");
	let overrides = $state<Record<string, CycleRollover>>({});
	let working = $state(false);
	let failure = $state<CycleFailure | null>(null);

	const destinationOf = $derived((issueId: string) => overrides[issueId] ?? rollover);

	const nextLabel = $derived(
		detail.kind === "ready" && detail.nextNumber !== null
			? `Cycle ${detail.nextNumber}`
			: "the next cycle"
	);

	function dismiss() {
		const next = new URL(page.url);
		next.searchParams.delete("close");
		goto(next, { replaceState: true, noScroll: true, keepFocus: true });
	}

	async function close() {
		if (detail.kind !== "ready") return;

		working = true;
		failure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/cycles/{cycleId}/close", {
				params: { path: { workspaceId: data.workspace.id, cycleId: detail.cycle.id } },
				body: {
					rollover: unfinished.length > 0 ? rollover : undefined,
					overrides: Object.entries(overrides).map(([issueId, destination]) => ({
						issueId,
						destination,
					})),
				},
			});

			if (error) {
				failure = readCycleFailure(error);

				return;
			}

			overrides = {};
			dismiss();
			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}
</script>

<svelte:head>
	<title>
		{detail.kind === "ready" ? detail.cycle.name : "Cycle"} · {data.workspace.name} · Norn
	</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Layers class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<a
				href={teamCyclesPath(slug, page.params.teamKey ?? "")}
				class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Cycles
			</a>
			{#if detail.kind === "ready"}
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
					{detail.cycle.name}
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
					<h2 class="text-md font-medium tracking-snug text-ink-900">No cycle here</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no cycle at this address, or it belongs to a team you cannot see.
					</p>
					<div>
						<Button
							variant="secondary"
							size="sm"
							href={teamCyclesPath(slug, page.params.teamKey ?? "")}
						>
							All cycles
						</Button>
					</div>
				</div>
			{:else if detail.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this cycle</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				<div class="flex flex-col gap-1">
					<Eyebrow class="text-ink-600">{phaseLabel(detail.cycle.phase)}</Eyebrow>
					<div class="flex flex-wrap items-baseline justify-between gap-2">
						<p class="font-mono text-md text-ink-900 tabular-nums">
							{cycleWindow(detail.cycle.startsOn, detail.cycle.endsOn)}
						</p>
						{#if progress}
							<ProgressBar {progress} class="lg:hidden" />
						{/if}
					</div>
					{#if detail.cycle.phase === "closed" && detail.cycle.closedAt}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Closed on {onCalendarDate(detail.cycle.closedAt.slice(0, 10))}. What happened in this
							cycle no longer changes.
						</p>
					{/if}
				</div>

				{#if failure}
					<Alert.Root variant="destructive">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>That did not go through</Alert.Title>
						<Alert.Description>{cycleFailureMessage(failure)}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if canClose && !closing}
					<Alert.Root variant="warning">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>This cycle has ended</Alert.Title>
						<Alert.Description>
							{#if unfinished.length > 0}
								{unfinished.length}
								{unfinished.length === 1 ? "issue is" : "issues are"} still unfinished. Closing asks
								where they go.
							{:else}
								Everything in it is finished. Close it to put it in the past.
							{/if}
						</Alert.Description>
						<Alert.Action>
							<Button variant="secondary" size="sm" href="{page.url.pathname}?close">
								Close this cycle
							</Button>
						</Alert.Action>
					</Alert.Root>
				{/if}

				{#if canClose && closing}
					<section class="flex flex-col gap-3 rounded-lg border border-line-strong bg-paper-1 p-3">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								Close {detail.cycle.name}
							</h2>
							{#if unfinished.length > 0}
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									{unfinished.length}
									{unfinished.length === 1 ? "issue is" : "issues are"} not finished. Decide where they
									go — you can send individual ones elsewhere before confirming.
								</p>
							{:else}
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									Nothing is unfinished, so there is nothing to move.
								</p>
							{/if}
						</div>

						{#if unfinished.length > 0}
							<div class="flex flex-col gap-1.5">
								<span class="text-sm text-ink-900" id="rollover-label">Where they go</span>
								<Select.Root
									type="single"
									value={rollover}
									onValueChange={(value) => (rollover = value as CycleRollover)}
									disabled={working}
								>
									<Select.Trigger class="w-full" aria-labelledby="rollover-label">
										{rollover === "next" ? `Move them to ${nextLabel}` : "Return them to the backlog"}
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="next">Move them to {nextLabel}</Select.Item>
										<Select.Item value="backlog">Return them to the backlog</Select.Item>
									</Select.Content>
								</Select.Root>
							</div>

							<ul class="flex flex-col gap-1.5">
								{#each unfinished as issue (issue.id)}
									<li class="flex flex-wrap items-center justify-between gap-2">
										<span class="flex min-w-0 items-center gap-2">
											<span class="font-mono text-xs text-muted-foreground tabular-nums">
												{issue.reference}
											</span>
											<span class="min-w-0 truncate text-sm text-ink-900">{issue.title}</span>
										</span>
										<Select.Root
											type="single"
											value={destinationOf(issue.id)}
											onValueChange={(value) =>
												(overrides = { ...overrides, [issue.id]: value as CycleRollover })}
											disabled={working}
										>
											<Select.Trigger
												class="w-40 shrink-0"
												aria-label="Where {issue.reference} goes"
											>
												{destinationOf(issue.id) === "next" ? nextLabel : "Backlog"}
											</Select.Trigger>
											<Select.Content>
												<Select.Item value="next">{nextLabel}</Select.Item>
												<Select.Item value="backlog">Backlog</Select.Item>
											</Select.Content>
										</Select.Root>
									</li>
								{/each}
							</ul>
						{/if}

						<div class="flex gap-2">
							<Button disabled={working} onclick={close}>
								{working ? "Closing" : "Close cycle"}
							</Button>
							<Button variant="secondary" disabled={working} onclick={dismiss}>Cancel</Button>
						</div>
					</section>
				{/if}

				<section class="flex flex-col gap-2">
					<div class="flex items-baseline justify-between gap-2">
						<Eyebrow class="text-ink-600">Scope</Eyebrow>
						<span class="font-mono text-xs text-muted-foreground tabular-nums">
							{joined.length} added · {removed.length} removed after it started
						</span>
					</div>

					{#if inScope.length === 0}
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							{#if detail.cycle.phase === "closed"}
								Nothing was left in this cycle when it closed.
							{:else}
								Nothing is in this cycle yet. Put an issue in it from the issue itself.
							{/if}
						</p>
					{:else}
						{#each groups as group (group.state.id)}
							<div class="flex flex-col">
								<div class="flex items-center gap-2 py-1.5">
									<StatusIcon category={group.state.category} name={group.state.name} decorative />
									<span class="text-sm text-ink-900">{group.state.name}</span>
									<span class="font-mono text-xs text-muted-foreground tabular-nums">
										{group.issues.length}
									</span>
								</div>
								<ul class="flex flex-col">
									{#each group.issues as issue (issue.id)}
										<li class="flex items-center gap-2">
											<IssueRow
												{issue}
												href={workspacePath(slug, `/issues/${issue.reference}`)}
												now={data.now}
												timezone={data.workspace.timezone}
												class="min-w-0 flex-1"
											/>
											{#if addedIds.has(issue.id)}
												<span
													class="shrink-0 text-xs text-muted-foreground"
													title="Added after this cycle started"
												>
													added later
												</span>
											{/if}
										</li>
									{/each}
								</ul>
							</div>
						{/each}
					{/if}
				</section>

				{#if removed.length > 0}
					<section class="flex flex-col gap-2">
						<Eyebrow class="text-ink-600">Taken out after it started</Eyebrow>
						<ul class="flex flex-col gap-1">
							{#each removed as change (change.id)}
								<li class="flex min-w-0 items-center gap-2">
									<span class="font-mono text-xs text-muted-foreground tabular-nums">
										{change.issueReference}
									</span>
									<a
										href={workspacePath(slug, `/issues/${change.issueReference}`)}
										class="min-w-0 truncate text-sm text-ink-900 transition-colors duration-110 ease-out hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
									>
										{change.issueTitle}
									</a>
								</li>
							{/each}
						</ul>
					</section>
				{/if}

				{#if movedOn.length > 0}
					<section class="flex flex-col gap-2">
						<Eyebrow class="text-ink-600">Moved on when it closed</Eyebrow>
						<ul class="flex flex-col gap-1">
							{#each movedOn as change (change.id)}
								<li class="flex min-w-0 items-center gap-2">
									<span class="font-mono text-xs text-muted-foreground tabular-nums">
										{change.issueReference}
									</span>
									<a
										href={workspacePath(slug, `/issues/${change.issueReference}`)}
										class="min-w-0 truncate text-sm text-ink-900 transition-colors duration-110 ease-out hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
									>
										{change.issueTitle}
									</a>
									<span class="ml-auto shrink-0 text-xs text-muted-foreground">
										{change.change === "rolled_over" ? "Next cycle" : "Backlog"}
									</span>
								</li>
							{/each}
						</ul>
					</section>
				{/if}
			{/if}
		</div>
	</div>
</div>
