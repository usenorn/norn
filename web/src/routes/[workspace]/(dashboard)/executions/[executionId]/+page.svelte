<script lang="ts">
	import { page } from "$app/state";
	import { goto, invalidate } from "$app/navigation";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { useRealtime } from "$lib/realtime/connection.svelte";
	import { waitingOnLine, waitingTitle, type DecisionRight } from "$lib/executions/reviews";
	import QuestionList from "$lib/questions/question-list.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import AttentionPanel from "$lib/components/norn/attention-panel.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import RunHeader from "$lib/executions/run-header.svelte";
	import RunActions from "$lib/executions/run-actions.svelte";
	import RunTimeline from "$lib/executions/run-timeline.svelte";
	import PlanPanel from "$lib/executions/plan-panel.svelte";
	import ChangesetPanel from "$lib/executions/changeset-panel.svelte";
	import ServicesPanel from "$lib/executions/services-panel.svelte";
	import PreviewsPanel from "$lib/executions/previews-panel.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import {
		blockingQuestion,
		changeStatLine,
		changeTotals,
		isSettled,
		mergeTimeline,
		publicationIncomplete,
		readRunFailure,
		retainLongerSeconds,
		runFailureMessage,
		timelinePageSize,
		timelinePreviewSize,
		type Execution,
		type ExecutionChangeSet,
		type ExecutionEvent,
		type IssueQuestion,
		type RunView,
	} from "$lib/executions/executions";
	import { runPreviewStates } from "./preview";
	import type { PageProps } from "./$types";
	import { attempt, unknownLine, type ApiResult, type Outcome } from "$lib/api/attempt";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? runPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const workspace = $derived(data.workspace);
	const run = $derived<RunView>(preview?.run ?? data.run);
	const ready = $derived(run.kind === "ready" ? run : undefined);

	let pushedRun = $state.raw<{ source: unknown; execution: Execution } | null>(null);
	let pushedTimeline = $state.raw<{ source: unknown; events: ExecutionEvent[] }>({
		source: null,
		events: [],
	});
	let readMore = $state.raw<{ source: unknown; events: ExecutionEvent[]; full: boolean }>({
		source: null,
		events: [],
		full: false,
	});
	let pushedChangeset = $state.raw<{ source: unknown; changeset: ExecutionChangeSet } | null>(null);
	let minted = $state.raw<{ run: string; held: Record<string, string> }>({ run: "", held: {} });

	let working = $state(false);
	let failure = $state<string | null>(null);
	let ticked = $state<string | null>(null);

	const execution = $derived(
		ready && pushedRun?.source === ready ? pushedRun.execution : ready?.execution
	);

	const timeline = $derived.by(() => {
		if (!ready) return [];

		const pushed = pushedTimeline.source === ready ? pushedTimeline.events : [];
		const read = readMore.source === ready ? readMore.events : [];

		return mergeTimeline(ready.timeline, [...read, ...pushed]);
	});

	const changeset = $derived(
		ready && pushedChangeset?.source === ready ? pushedChangeset.changeset : ready?.changeset
	);
	const reviewHref = $derived(
		execution ? workspacePath(workspace.slug, `/executions/${execution.id}/review`) : ""
	);
	const plans = $derived(ready?.plans ?? []);
	const plansReach = $derived(ready?.plansReach ?? "loaded");
	const questionsReach = $derived(ready?.questionsReach ?? "loaded");
	const reviewLinks = $derived(
		execution
			? {
					issue: new URL(
						workspacePath(workspace.slug, `/issues/${execution.issueReference}`),
						page.url.origin
					).href,
					run: new URL(workspacePath(workspace.slug, `/executions/${execution.id}`), page.url.origin)
						.href,
				}
			: { issue: "", run: "" }
	);
	const shownMinted = $derived(minted.run === execution?.id ? minted.held : {});

	const questions = $derived(ready?.questions ?? []);
	const right = $derived<DecisionRight>(ready?.right ?? { canDecide: false });
	const asking = $derived(blockingQuestion(questions));
	const stalled = $derived(execution ? publicationIncomplete(execution, changeset) : false);
	const now = $derived(ticked ?? data.now);
	const live = $derived(Boolean(execution) && !isSettled(execution!.state));
	const moreTimeline = $derived.by(() => {
		if (!ready) return false;
		if (readMore.source === ready) return readMore.full;

		return ready.timeline.length >= timelinePreviewSize;
	});

	const realtime = useRealtime();

	$effect(() => {
		if (!realtime || !ready) return;

		const openRun = ready.execution.id;
		const source = ready;

		return realtime.on((event) => {
			if (event.kind === "execution.updated") {
				const moved = event.payload as Execution;

				if (moved.id !== openRun) return;

				pushedRun = { source, execution: moved };

				return;
			}

			if (event.kind === "execution.changeset") {
				const reported = event.payload as ExecutionChangeSet;

				if (reported.executionId !== openRun) return;

				pushedChangeset = { source, changeset: reported };

				return;
			}

			if (event.kind === "execution.event") {
				const entry = event.payload as ExecutionEvent;

				if (entry.executionId !== openRun) return;

				const held = pushedTimeline.source === source ? pushedTimeline.events : [];

				pushedTimeline = { source, events: [...held, entry] };

				return;
			}

			if (event.kind === "execution.plan") {
				const planned = event.payload as Execution;

				if (planned.id !== openRun) return;

				realtime.refetch(keys.execution(openRun));

				return;
			}

			if (event.kind === "question.asked" || event.kind === "question.settled") {
				const asked = event.payload as IssueQuestion;

				if (asked.executionId !== openRun) return;

				realtime.refetch(keys.execution(openRun));
			}
		});
	});

	$effect(() => {
		if (!live) return;

		const timer = setInterval(() => (ticked = new Date().toISOString()), 1000);

		return () => clearInterval(timer);
	});

	function failureLine(outcome: Outcome<unknown>): string {
		return outcome.kind === "refused"
			? runFailureMessage(readRunFailure(outcome.problem))
			: unknownLine;
	}

	async function act(run: () => Promise<ApiResult<unknown>>): Promise<boolean> {
		working = true;
		failure = null;

		const outcome = await attempt({ run });

		working = false;

		if (outcome.kind !== "done") {
			failure = failureLine(outcome);

			return false;
		}

		await invalidate(keys.execution(execution!.id));

		return true;
	}

	function pathOf(executionId: string) {
		return { workspaceId: workspace.id, executionId };
	}

	function cancel() {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/cancel", {
				params: { path: pathOf(execution!.id) },
				body: {},
			})
		);
	}

	function retryPublication() {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/publication/retry", {
				params: { path: pathOf(execution!.id) },
			})
		);
	}

	function abandonPublication() {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/publication/abandon", {
				params: { path: pathOf(execution!.id) },
			})
		);
	}

	function retain() {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/retain", {
				params: { path: pathOf(execution!.id) },
				body: { longerSeconds: retainLongerSeconds },
			})
		);
	}

	function approvePlan(revision: number) {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/plans/{revision}/approve", {
				params: { path: { ...pathOf(execution!.id), revision } },
			})
		);
	}

	function revisePlan(revision: number, feedback: string): Promise<boolean> {
		return act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/plans/{revision}/revise", {
				params: { path: { ...pathOf(execution!.id), revision } },
				body: { feedback },
			})
		);
	}

	async function share(previewName: string, lifetimeSeconds: number, passcode: string) {
		working = true;
		failure = null;

		const outcome = await attempt({
			run: () =>
				api.POST(
					"/workspaces/{workspaceId}/executions/{executionId}/previews/{previewName}/share",
					{
						params: { path: { ...pathOf(execution!.id), previewName } },
						body: { lifetimeSeconds, ...(passcode === "" ? {} : { passcode }) },
					}
				),
		});

		working = false;

		if (outcome.kind !== "done") {
			failure = failureLine(outcome);

			return;
		}

		minted = { run: execution!.id, held: { ...shownMinted, [previewName]: outcome.value.url } };

		await invalidate(keys.execution(execution!.id));
	}

	function revoke(previewName: string, shareLinkId: string) {
		const held = { ...shownMinted };

		delete held[previewName];

		minted = { run: execution!.id, held };

		void act(() =>
			api.DELETE(
				"/workspaces/{workspaceId}/executions/{executionId}/previews/{previewName}/share/{shareLinkId}",
				{ params: { path: { ...pathOf(execution!.id), previewName, shareLinkId } } }
			)
		);
	}

	function downloadOf(artifactId: string) {
		return `/v1/workspaces/${workspace.id}/executions/${execution!.id}/artifacts/${artifactId}/content`;
	}

	async function restart() {
		working = true;
		failure = null;

		const outcome = await attempt({
			run: () =>
				api.POST("/workspaces/{workspaceId}/executions/{executionId}/restart", {
					params: { path: pathOf(execution!.id) },
				}),
		});

		working = false;

		if (outcome.kind !== "done") {
			failure = failureLine(outcome);

			return;
		}

		await goto(workspacePath(workspace.slug, `/executions/${outcome.value.id}`));
	}

	function answer(question: IssueQuestion, given: string) {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/issues/{issueId}/questions/{questionId}/answer", {
				params: {
					path: {
						workspaceId: workspace.id,
						issueId: question.issueId,
						questionId: question.id,
					},
				},
				body: { answer: given },
			})
		);
	}

	function dismiss(question: IssueQuestion) {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/issues/{issueId}/questions/{questionId}/dismiss", {
				params: {
					path: {
						workspaceId: workspace.id,
						issueId: question.issueId,
						questionId: question.id,
					},
				},
			})
		);
	}

	async function readOnTimeline() {
		if (!ready || working) return;

		working = true;

		try {
			const { data: next } = await api.GET(
				"/workspaces/{workspaceId}/executions/{executionId}/timeline",
				{
					params: {
						path: pathOf(ready.execution.id),
						query: { after: timeline.at(-1)?.sequence, limit: timelinePageSize },
					},
				}
			);

			if (!next) return;

			const held = readMore.source === ready ? readMore.events : [];

			readMore = {
				source: ready,
				events: [...held, ...next],
				full: next.length >= timelinePageSize,
			};
		} finally {
			working = false;
		}
	}

</script>

<svelte:head>
	<title>
		{execution ? `${execution.reference} · ` : ""}Run · {workspace.name} · Norn
	</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			{#if execution}
				<a
					href={workspacePath(workspace.slug, `/issues/${execution.issueReference}`)}
					class="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
				>
					<ArrowLeft aria-hidden="true" class="size-3.5" />
					{execution.issueReference}
				</a>
			{:else}
				<a
					href={workspacePath(workspace.slug, "/issues")}
					class="inline-flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground"
				>
					<ArrowLeft aria-hidden="true" class="size-3.5" />
					Issues
				</a>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if run.kind === "loading"}
				<p class="my-auto text-sm text-muted-foreground">Reading this run…</p>
			{:else if run.kind === "not_found"}
				<div class="my-auto flex flex-col gap-2">
					<h1 class="text-lg text-ink-900">No such run</h1>
					<p class="text-sm text-muted-foreground">
						This run is not here. It may have been on an issue you cannot see.
					</p>
					<a
						href={workspacePath(workspace.slug, "/issues")}
						class="text-sm text-ink-900 underline underline-offset-2"
					>
						Back to issues
					</a>
				</div>
			{:else if run.kind === "unavailable"}
				<div class="my-auto">
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" class="size-4" />
						<Alert.Title>We could not load this run</Alert.Title>
						<Alert.Description>
							Something went wrong and nothing changed. Wait a moment and try again.
						</Alert.Description>
					</Alert.Root>
				</div>
			{:else if execution}
				<RunHeader {execution} runner={run.runner} {now} />

				{#if failure}
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" class="size-4" />
						<Alert.Title>That did not work</Alert.Title>
						<Alert.Description>{failure}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if asking}
					<AttentionPanel label="A question is waiting" title="The run is waiting on you">
						<QuestionList
							questions={[asking]}
							timezone={workspace.timezone}
							canAnswer={right.canDecide}
							refusal={waitingOnLine(right)}
							{working}
							onanswer={answer}
							ondismiss={dismiss}
						/>
					</AttentionPanel>
				{:else if execution.state === "awaiting_plan_approval"}
					<AttentionPanel label="A plan is waiting" title={waitingTitle("plan", right)}>
						<p class="max-w-prose text-sm leading-normal text-ink-900 text-pretty">
							Read the plan and approve it, or ask for changes. The coding agent builds nothing until
							somebody approves it.
						</p>
						<div>
							<Button href="#plan" size="sm">Read the plan</Button>
						</div>
					</AttentionPanel>
				{:else if execution.state === "awaiting_review"}
					<AttentionPanel label="Changes are waiting" title={waitingTitle("changes", right)}>
						<p class="max-w-prose text-sm leading-normal text-ink-900 text-pretty">
							Review what the run changed, comment on any line, then approve it or send it back.
							Nothing is pushed until somebody approves.
						</p>
						{#if changeset && changeset.repositories.length > 0}
							<p class="font-mono text-xs text-muted-foreground">
								{changeStatLine(changeTotals(changeset.repositories))}
							</p>
						{/if}
						<div>
							<Button href={reviewHref} size="sm">Review the changes</Button>
						</div>
					</AttentionPanel>
				{:else if stalled}
					<AttentionPanel label="Publication is incomplete" title={waitingTitle("publication", right)}>
						<p class="max-w-prose text-sm leading-normal text-ink-900 text-pretty">
							Some repositories were not published. Retry publishes only what is left of the approved
							changes. Give up leaves the run failed.
						</p>
						{#if right.canDecide}
							<div class="flex flex-wrap items-center gap-2">
								<Button size="sm" disabled={working} onclick={retryPublication}>
									Retry publication
								</Button>
								<Button variant="secondary" size="sm" disabled={working} onclick={abandonPublication}>
									Give up
								</Button>
							</div>
						{:else}
							<p class="text-xs text-muted-foreground">{waitingOnLine(right)}</p>
						{/if}
					</AttentionPanel>
				{/if}

				{#if execution.stage === "planning" || plans.length > 0 || plansReach === "unavailable"}
					<PlanPanel
						{execution}
						{plans}
						{plansReach}
						{questions}
						{questionsReach}
						{right}
						timezone={workspace.timezone}
						links={reviewLinks}
						{working}
						onapprove={approvePlan}
						onrevise={revisePlan}
						onretry={() => void invalidate(keys.execution(execution.id))}
					/>
				{/if}

				<RunActions
					{execution}
					{working}
					{now}
					timezone={workspace.timezone}
					oncancel={cancel}
					onrestart={restart}
					onretain={retain}
				/>

				<ChangesetPanel
					{execution}
					{changeset}
					links={run.codeLinks}
					review={reviewHref}
					{downloadOf}
				/>

				<RunTimeline
					{timeline}
					timezone={workspace.timezone}
					more={moreTimeline}
					{working}
					onmore={readOnTimeline}
				/>

				<ServicesPanel
					{execution}
					services={run.services}
				/>

				<PreviewsPanel
					{execution}
					previews={run.previews}
					runner={run.runner}
					minted={shownMinted}
					{working}
					{now}
					timezone={workspace.timezone}
					onshare={share}
					onrevoke={revoke}
				/>

				{#if questions.length > (asking ? 1 : 0)}
					<section class="flex min-w-0 flex-col gap-2" aria-label="Questions">
						<Eyebrow rule>Questions</Eyebrow>
						<QuestionList
							questions={questions.filter((question) => question.id !== asking?.id)}
							timezone={workspace.timezone}
							canAnswer={right.canDecide}
							refusal={waitingOnLine(right)}
							{working}
							onanswer={answer}
							ondismiss={dismiss}
						/>
					</section>
				{/if}
			{/if}
		</div>
	</div>
</div>
