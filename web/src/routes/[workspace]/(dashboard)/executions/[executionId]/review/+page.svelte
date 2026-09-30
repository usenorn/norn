<script lang="ts">
	import { page } from "$app/state";
	import { invalidate } from "$app/navigation";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import FolderTree from "@lucide/svelte/icons/folder-tree";
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Sheet from "$lib/components/ui/sheet/index.js";
	import * as ToggleGroup from "$lib/components/ui/toggle-group/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Markdown from "$lib/issues/markdown.svelte";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { attempt, unknownLine, type ApiResult } from "$lib/api/attempt";
	import { useRealtime } from "$lib/realtime/connection.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import { onDateAndTime } from "$lib/time";
	import RunState from "$lib/executions/run-state.svelte";
	import ReviewFile from "$lib/executions/review-file.svelte";
	import ReviewFiles from "$lib/executions/review-files.svelte";
	import { waitingOnLine } from "$lib/executions/reviews";
	import ReviewSubmit from "$lib/executions/review-submit.svelte";
	import ReviewCommits from "$lib/executions/review-commits.svelte";
	import ReviewPreviews from "$lib/executions/review-previews.svelte";
	import ReviewRevisions from "$lib/executions/review-revisions.svelte";
	import {
		blockingQuestion,
		changeStatLine,
		readRunFailure,
		runFailureMessage,
		type Execution,
	} from "$lib/executions/executions";
	import type { DiffAnchor } from "$lib/executions/diff";
	import {
		draftCount,
		fileId,
		olderRevisionLine,
		questionsOpenLine,
		readingLatest,
		repositoryNote,
		reviewClosedLine,
		reviewOpen,
		revisionLabel,
		snapshotTotals,
		threadsOf,
		threadsOn,
		verdictLabel,
		viewedKey,
		type ListedRepository,
		type ReviewLayout,
		type ReviewVerdict,
		type ReviewView,
		type ReviewedRepository,
	} from "$lib/executions/review";
	import { reviewPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? reviewPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const workspace = $derived(data.workspace);
	const view = $derived<ReviewView>(preview?.review ?? data.review);
	const ready = $derived(view.kind === "ready" ? view : undefined);
	const execution = $derived(ready?.execution);

	const layoutKey = "norn:review-layout";

	let layout = $state<ReviewLayout>("unified");
	let viewed = $state<Record<string, boolean>>({});
	let working = $state(false);
	let failure = $state<string | null>(null);
	let filesOpen = $state(false);

	const latest = $derived(ready ? readingLatest(ready.review) : true);
	const open = $derived(Boolean(execution) && reviewOpen(execution!) && latest);
	const threads = $derived(threadsOf(ready?.review.comments ?? []));
	const drafts = $derived(draftCount(ready?.review.comments ?? []));
	const locked = $derived(
		!ready
			? undefined
			: !ready.right.canDecide
				? waitingOnLine(ready.right)
				: blockingQuestion(ready.questions)
					? questionsOpenLine
					: undefined
	);
	const runHref = $derived(
		execution ? workspacePath(workspace.slug, `/executions/${execution.id}`) : ""
	);
	const totals = $derived(snapshotTotals(ready?.review.repositories ?? []));

	const listed = $derived<ListedRepository[]>(
		(ready?.repositories ?? []).map((repository, repositoryIndex) => ({
			repository: repository.repository,
			note: repositoryNote(repository),
			files:
				repository.diff.kind === "ready"
					? repository.diff.files.map((file, index) => ({
							id: fileId(repositoryIndex, index),
							file,
							threads: threadsOn(threads, repository.repository, file.path).length,
							viewed: isViewed(repository, file.path),
						}))
					: [],
		}))
	);

	function read(key: string): string | null {
		try {
			return localStorage.getItem(key);
		} catch {
			return null;
		}
	}

	function write(key: string, value: string | null) {
		try {
			if (value === null) localStorage.removeItem(key);
			else localStorage.setItem(key, value);
		} catch {
			return;
		}
	}

	$effect(() => {
		if (read(layoutKey) === "split") layout = "split";
	});

	function isViewed(repository: ReviewedRepository, path: string): boolean {
		if (!execution) return false;

		const key = viewedKey(execution.id, repository, path);

		return viewed[key] ?? read(key) === "1";
	}

	function markViewed(repository: ReviewedRepository, path: string, seen: boolean) {
		const key = viewedKey(execution!.id, repository, path);

		viewed = { ...viewed, [key]: seen };
		write(key, seen ? "1" : null);
	}

	function chooseLayout(chosen: string) {
		if (chosen !== "unified" && chosen !== "split") return;

		layout = chosen;
		write(layoutKey, chosen);
	}

	const realtime = useRealtime();

	$effect(() => {
		if (!realtime || !ready) return;

		const openRun = ready.execution.id;

		return realtime.on((event) => {
			if (
				event.kind !== "execution.review" &&
				event.kind !== "execution.changeset" &&
				event.kind !== "execution.updated"
			) {
				return;
			}

			const touched = event.payload as { id?: string; executionId?: string };

			if ((touched.id ?? touched.executionId) !== openRun) return;

			realtime.refetch(keys.executionReview(openRun));
		});
	});

	async function act(run: () => Promise<ApiResult<unknown>>): Promise<boolean> {
		working = true;
		failure = null;

		const outcome = await attempt({ run });

		working = false;

		if (outcome.kind !== "done") {
			failure =
				outcome.kind === "refused" ? runFailureMessage(readRunFailure(outcome.problem)) : unknownLine;

			return false;
		}

		await invalidate(keys.executionReview(execution!.id));

		return true;
	}

	function pathOf(current: Execution) {
		return { workspaceId: workspace.id, executionId: current.id };
	}

	function comment(
		repository: ReviewedRepository,
		path: string,
		anchor: DiffAnchor,
		hunk: string,
		body: string,
		publish: boolean
	): Promise<boolean> {
		return act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/review/comments", {
				params: { path: pathOf(execution!) },
				body: {
					repository: repository.repository,
					path,
					side: anchor.side,
					line: anchor.line,
					hunk,
					body,
					publish,
				},
			})
		);
	}

	function reply(parentId: string, body: string, publish: boolean): Promise<boolean> {
		return act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/review/comments", {
				params: { path: pathOf(execution!) },
				body: { parentId, body, publish },
			})
		);
	}

	function edit(reviewCommentId: string, body: string): Promise<boolean> {
		return act(() =>
			api.PATCH(
				"/workspaces/{workspaceId}/executions/{executionId}/review/comments/{reviewCommentId}",
				{ params: { path: { ...pathOf(execution!), reviewCommentId } }, body: { body } }
			)
		);
	}

	function remove(reviewCommentId: string) {
		void act(() =>
			api.DELETE(
				"/workspaces/{workspaceId}/executions/{executionId}/review/comments/{reviewCommentId}",
				{ params: { path: { ...pathOf(execution!), reviewCommentId } } }
			)
		);
	}

	function resolve(reviewCommentId: string, resolved: boolean) {
		void act(() =>
			api.POST(
				"/workspaces/{workspaceId}/executions/{executionId}/review/comments/{reviewCommentId}/resolve",
				{ params: { path: { ...pathOf(execution!), reviewCommentId } }, body: { resolved } }
			)
		);
	}

	function submit(verdict: ReviewVerdict, summary: string): Promise<boolean> {
		return act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/reviews", {
				params: { path: pathOf(execution!) },
				body: { verdict, summary, heads: ready!.review.heads },
			})
		);
	}
</script>

<svelte:head>
	<title>
		{execution ? `Review ${execution.reference} · ` : "Review · "}{workspace.name} · Norn
	</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<a
				href={execution ? runHref : workspacePath(workspace.slug, "/reviews")}
				class="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
			>
				<ArrowLeft aria-hidden="true" class="size-3.5" />
				{execution ? `Run ${execution.reference}` : "Reviews"}
			</a>
		</div>
	</div>

	<div class="flex-1 overflow-auto [--review-bar:--spacing(13)]">
		{#if view.kind === "loading"}
			<p class="mx-auto my-10 w-full max-w-180 px-4 text-sm text-muted-foreground">Reading the changes…</p>
		{:else if view.kind === "not_found"}
			<div class="mx-auto my-10 flex w-full max-w-180 flex-col gap-2 px-4">
				<h1 class="text-lg text-ink-900">No such run</h1>
				<p class="text-sm text-muted-foreground">
					This run is not here. It may have been on an issue you cannot see.
				</p>
				<a href={workspacePath(workspace.slug, "/reviews")} class="text-sm text-ink-900 underline underline-offset-2">
					Back to reviews
				</a>
			</div>
		{:else if view.kind === "revision_not_found"}
			<div class="mx-auto my-10 flex w-full max-w-180 flex-col gap-2 px-4">
				<h1 class="text-lg text-ink-900">No such revision</h1>
				<p class="text-sm text-muted-foreground">
					This run never finished a pass with that number.
				</p>
				<a
					href={workspacePath(workspace.slug, `/executions/${view.executionId}/review`)}
					class="text-sm text-ink-900 underline underline-offset-2"
				>
					Read the latest revision
				</a>
			</div>
		{:else if view.kind === "unavailable"}
			<div class="mx-auto my-10 w-full max-w-180 px-4">
				<Alert.Root variant="destructive">
					<CircleAlert aria-hidden="true" class="size-4" />
					<Alert.Title>We could not load these changes</Alert.Title>
					<Alert.Description>Something went wrong and nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			</div>
		{:else if ready && execution}
			<div
				class="sticky top-0 z-20 flex h-(--review-bar) items-center gap-3 border-b border-line-default bg-background px-4"
			>
				<div class="flex min-w-0 flex-1 items-baseline gap-x-3 overflow-hidden">
					<h1 class="shrink-0 font-mono text-sm text-ink-900">{execution.reference}</h1>
					<RunState state={execution.state} />
					{#if ready.review.revision > 0}
						<span class="shrink-0 font-mono text-2xs text-muted-foreground">
							{revisionLabel(ready.review.revision)}
						</span>
					{/if}
					<span class="hidden truncate font-mono text-2xs text-muted-foreground sm:inline">
						{changeStatLine(totals)}
					</span>
				</div>

				<Button
					variant="outline"
					size="sm"
					class="lg:hidden"
					onclick={() => (filesOpen = true)}
					aria-label="Changed files"
				>
					<FolderTree aria-hidden="true" />
					<span class="hidden sm:inline">Files</span>
				</Button>

				<ToggleGroup.Root
					type="single"
					value={layout}
					onValueChange={chooseLayout}
					variant="outline"
					size="sm"
					class="hidden md:flex"
					aria-label="How the diff is laid out"
				>
					<ToggleGroup.Item value="unified">Unified</ToggleGroup.Item>
					<ToggleGroup.Item value="split">Split</ToggleGroup.Item>
				</ToggleGroup.Root>

				{#if open}
					<ReviewSubmit {drafts} {working} {locked} onsubmit={submit} />
				{/if}
			</div>

			<div
				class="mx-auto grid w-full max-w-360 grid-cols-1 gap-6 px-4 py-4 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))] lg:grid-cols-[15rem_minmax(0,1fr)]"
			>
				<aside class="hidden lg:block">
					<div class="sticky top-[calc(var(--review-bar)+--spacing(4))] max-h-[calc(100dvh-var(--review-bar)-6rem)] overflow-y-auto">
						<ReviewFiles repositories={listed} />
					</div>
				</aside>

				<div class="flex min-w-0 flex-col gap-4">
					{#if failure}
						<Alert.Root variant="destructive">
							<CircleAlert aria-hidden="true" class="size-4" />
							<Alert.Title>That did not work</Alert.Title>
							<Alert.Description>{failure}</Alert.Description>
						</Alert.Root>
					{/if}

					{#if !latest}
						<p class="rounded-sm border border-line-default bg-paper-1 px-3 py-2 text-sm text-ink-900 text-pretty">
							{olderRevisionLine(ready.review)}
							<a href="?" class="underline underline-offset-2">Read the latest</a>
						</p>
					{:else if !open}
						<p class="rounded-sm border border-line-default bg-paper-1 px-3 py-2 text-sm text-ink-900 text-pretty">
							{reviewClosedLine(execution)}
							<a href={runHref} class="underline underline-offset-2">Follow the run</a>
						</p>
					{:else}
						<p class="max-w-prose text-sm leading-normal text-muted-foreground text-pretty">
							Nothing here has been pushed. Comment on any line; your comments stay drafts until you finish
							your review. Approving pushes the branch and opens the pull request.
						</p>
					{/if}

					{#if ready.review.revisions.length > 1}
						<ReviewRevisions
							revisions={ready.review.revisions}
							current={ready.review.revision}
							latest={ready.review.latestRevision}
							timezone={workspace.timezone}
						/>
					{/if}

					{#if ready.review.summary}
						<section class="flex min-w-0 flex-col gap-1.5" aria-label="What the coding agent said">
							<Eyebrow rule>What the coding agent said</Eyebrow>
							<p class="max-w-prose text-sm leading-normal break-words text-ink-900 text-pretty">
								{ready.review.summary}
							</p>
						</section>
					{/if}

					{#if ready.review.revision > 0}
						<ReviewPreviews review={ready.review} />
					{/if}

					{#if ready.review.reviews.length > 0}
						<section class="flex min-w-0 flex-col gap-2" aria-label="Reviews so far">
							<Eyebrow rule>Reviews so far</Eyebrow>
							<ol class="flex min-w-0 flex-col gap-2">
								{#each ready.review.reviews as held (held.id)}
									<li class="flex min-w-0 flex-col gap-1 rounded-sm border border-line-default bg-card px-3 py-2">
										<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
											<span class="text-xs font-medium text-ink-900">{held.authorName || "Somebody"}</span>
											<Eyebrow
												tone={held.verdict === "approve" ? "success" : held.verdict === "request_changes" ? "attention" : "muted"}
											>
												{verdictLabel(held.verdict)}
											</Eyebrow>
											<span class="font-mono text-2xs text-muted-foreground">{revisionLabel(held.revision)}</span>
											<time class="font-mono text-2xs text-muted-foreground" datetime={held.submittedAt}>
												{onDateAndTime(held.submittedAt, workspace.timezone)}
											</time>
										</div>
										{#if held.summary}
											<Markdown source={held.summary} class="text-sm" />
										{/if}
									</li>
								{/each}
							</ol>
						</section>
					{/if}

					{#if ready.repositories.length === 0}
						<p class="py-6 text-sm text-muted-foreground">
							The run reported no repository it changed, so there is nothing to review line by line.
						</p>
					{/if}

					{#each ready.repositories as repository, repositoryIndex (repository.repository)}
						<section class="flex min-w-0 flex-col gap-3" aria-label={repository.repository}>
							<div class="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
								<h2 class="font-mono text-sm text-ink-900">{repository.repository}</h2>
								{#if repository.branch}
									<span class="inline-flex min-w-0 items-center gap-1 font-mono text-2xs break-all text-muted-foreground">
										<GitBranch aria-hidden="true" class="size-3 shrink-0" />
										{repository.branch}
									</span>
								{/if}
								<span class="font-mono text-2xs whitespace-nowrap">
									<span class="text-success">+{repository.additions}</span>
									<span class="text-destructive">−{repository.deletions}</span>
								</span>
							</div>

							{#if repository.commits.length > 0}
								<ReviewCommits {repository} />
							{/if}

							{#if repositoryNote(repository)}
								<p class="text-xs text-muted-foreground">
									{repositoryNote(repository)}
									{#if repository.artifactId}
										<a
											href={`/v1/workspaces/${workspace.id}/executions/${execution.id}/artifacts/${repository.artifactId}/content`}
											class="underline underline-offset-2 hover:text-foreground"
										>
											Download the full diff
										</a>
									{/if}
								</p>
							{/if}

							{#if repository.diff.kind === "ready"}
								{#each repository.diff.files as file, index (fileId(repositoryIndex, index))}
									<ReviewFile
										id={fileId(repositoryIndex, index)}
										{file}
										threads={threadsOn(threads, repository.repository, file.path)}
										{layout}
										viewed={isViewed(repository, file.path)}
										{open}
										{working}
										timezone={workspace.timezone}
										onviewed={(seen) => markViewed(repository, file.path, seen)}
										oncomment={(anchor, hunk, body, publish) =>
											comment(repository, file.path, anchor, hunk, body, publish)}
										onreply={reply}
										onedit={edit}
										ondelete={remove}
										onresolve={resolve}
									/>
								{/each}
							{/if}
						</section>
					{/each}
				</div>
			</div>

			<Sheet.Root bind:open={filesOpen}>
				<Sheet.Content side="left" class="w-[min(20rem,85vw)] overflow-y-auto p-4">
					<Sheet.Header class="p-0">
						<Sheet.Title>Changed files</Sheet.Title>
					</Sheet.Header>
					<ReviewFiles repositories={listed} onpick={() => (filesOpen = false)} />
				</Sheet.Content>
			</Sheet.Root>
		{/if}
	</div>
</div>
