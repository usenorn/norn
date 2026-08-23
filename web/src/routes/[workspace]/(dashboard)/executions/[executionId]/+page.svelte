<script lang="ts">
	import { page } from "$app/state";
	import { goto, invalidate } from "$app/navigation";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import { useRealtime } from "$lib/realtime/connection.svelte";
	import QuestionList from "$lib/questions/question-list.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import RunHeader from "$lib/executions/run-header.svelte";
	import RunActions from "$lib/executions/run-actions.svelte";
	import RunTimeline from "$lib/executions/run-timeline.svelte";
	import ServicesPanel from "$lib/executions/services-panel.svelte";
	import PreviewsPanel from "$lib/executions/previews-panel.svelte";
	import Transcript from "$lib/executions/transcript.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import {
		blockingQuestion,
		chunkPageSize,
		isSettled,
		mergeTimeline,
		readRunFailure,
		runFailureMessage,
		timelinePageSize,
		type Execution,
		type ExecutionEvent,
		type IssueQuestion,
		type RunCost,
		type RunView,
	} from "$lib/executions/executions";
	import { runPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? runPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const workspace = $derived(data.workspace);
	const run = $derived<RunView>(preview?.run ?? data.run);
	const ready = $derived(run.kind === "ready" ? run : undefined);
	const cost = $derived<RunCost>(preview?.cost ?? { kind: "unrecorded" });

	let pushedRun = $state.raw<{ source: unknown; execution: Execution } | null>(null);
	let pushedTimeline = $state.raw<{ source: unknown; events: ExecutionEvent[] }>({
		source: null,
		events: [],
	});
	let readMore = $state.raw<{ source: unknown; events: ExecutionEvent[] }>({
		source: null,
		events: [],
	});
	let readTranscript = $state.raw<{ source: unknown; entries: unknown[]; cursor?: number }>({
		source: null,
		entries: [],
	});

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

	const transcript = $derived.by(() => {
		if (!ready) return [];

		const read = readTranscript.source === ready ? readTranscript.entries : [];

		return [...ready.transcript, ...read] as typeof ready.transcript;
	});

	const questions = $derived(ready?.questions ?? []);
	const asking = $derived(blockingQuestion(questions));
	const now = $derived(ticked ?? data.now);
	const live = $derived(Boolean(execution) && !isSettled(execution!.state));
	const moreTimeline = $derived(timeline.length >= timelinePageSize);
	const moreTranscript = $derived(
		(readTranscript.source === ready ? readTranscript.cursor : ready?.transcriptCursor) !== undefined
	);

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

			if (event.kind === "execution.event") {
				const entry = event.payload as ExecutionEvent;

				if (entry.executionId !== openRun) return;

				const held = pushedTimeline.source === source ? pushedTimeline.events : [];

				pushedTimeline = { source, events: [...held, entry] };

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

	async function act(call: () => Promise<{ error?: unknown }>) {
		working = true;
		failure = null;

		try {
			const { error } = await call();

			if (error) {
				failure = runFailureMessage(readRunFailure(error));

				return;
			}

			await invalidate(keys.execution(execution!.id));
		} catch {
			failure = runFailureMessage({ kind: "unavailable" });
		} finally {
			working = false;
		}
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

	function retain() {
		void act(() =>
			api.POST("/workspaces/{workspaceId}/executions/{executionId}/retain", {
				params: { path: pathOf(execution!.id) },
				body: { longerSeconds: 3600 },
			})
		);
	}

	async function restart() {
		working = true;
		failure = null;

		try {
			const { data: started, error } = await api.POST(
				"/workspaces/{workspaceId}/executions/{executionId}/restart",
				{ params: { path: pathOf(execution!.id) } }
			);

			if (error || !started) {
				failure = runFailureMessage(readRunFailure(error));

				return;
			}

			await goto(workspacePath(workspace.slug, `/executions/${started.id}`));
		} catch {
			failure = runFailureMessage({ kind: "unavailable" });
		} finally {
			working = false;
		}
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

			readMore = { source: ready, events: [...held, ...next] };
		} finally {
			working = false;
		}
	}

	async function readOnTranscript() {
		if (!ready || working) return;

		working = true;

		try {
			const after =
				readTranscript.source === ready ? readTranscript.cursor : ready.transcriptCursor;

			const { data: next } = await api.GET(
				"/workspaces/{workspaceId}/executions/{executionId}/transcript",
				{ params: { path: pathOf(ready.execution.id), query: { after, limit: chunkPageSize } } }
			);

			if (!next) return;

			const held = readTranscript.source === ready ? readTranscript.entries : [];

			readTranscript = {
				source: ready,
				entries: [...held, ...next.flatMap((chunk) => chunk.entries)],
				cursor: next.at(-1)?.sequence,
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
				<RunHeader {execution} runner={run.runner} {cost} {now} />

				{#if failure}
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" class="size-4" />
						<Alert.Title>That did not work</Alert.Title>
						<Alert.Description>{failure}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if asking}
					<section
						class="flex min-w-0 flex-col gap-2 rounded-sm border border-amber-700/40 bg-amber-500/5 p-3 dark:border-amber-400/30"
						aria-label="A question is waiting"
					>
						<Eyebrow class="text-amber-700 dark:text-amber-400">The run is waiting on you</Eyebrow>
						<QuestionList
							questions={[asking]}
							timezone={workspace.timezone}
							canAnswer={true}
							{working}
							onanswer={answer}
							ondismiss={dismiss}
						/>
					</section>
				{/if}

				<RunActions
					{execution}
					{working}
					oncancel={cancel}
					onrestart={restart}
					onretain={retain}
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
					logs={run.logs}
					timezone={workspace.timezone}
				/>

				<PreviewsPanel {execution} previews={run.previews} runner={run.runner} />

				<Transcript
					{transcript}
					timezone={workspace.timezone}
					more={moreTranscript}
					{working}
					onmore={readOnTranscript}
				/>

				{#if questions.length > (asking ? 1 : 0)}
					<section class="flex min-w-0 flex-col gap-2" aria-label="Questions">
						<Eyebrow rule>Questions</Eyebrow>
						<QuestionList
							questions={questions.filter((question) => question.id !== asking?.id)}
							timezone={workspace.timezone}
							canAnswer={true}
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
