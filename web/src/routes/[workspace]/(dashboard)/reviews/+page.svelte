<script lang="ts">
	import { page } from "$app/state";
	import { invalidate } from "$app/navigation";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Retry from "$lib/components/norn/retry.svelte";
	import { api } from "$lib/api";
	import { attempt } from "$lib/api/attempt";
	import { keys } from "$lib/api/keys";
	import ReviewQuestionRow from "$lib/executions/review-question.svelte";
	import ReviewRunRow from "$lib/executions/review-run.svelte";
	import { readRunFailure, runFailureMessage } from "$lib/executions/executions";
	import {
		implementationQuestions,
		noReviewsLine,
		planningQuestions,
		waitingCount,
		waitingTotal,
		type ReviewQueue,
	} from "$lib/executions/reviews";
	import type { IssueQuestion } from "$lib/questions/questions";
	import { workspacePath } from "$lib/workspace/navigation";
	import { reviewPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? reviewPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const workspace = $derived(data.workspace);
	const queue = $derived<ReviewQueue>(preview?.queue ?? data.queue);

	let working = $state(false);
	let failure = $state<string | null>(null);

	function issueHref(reference: string): string {
		return workspacePath(workspace.slug, `/issues/${reference}`);
	}

	async function settle(question: IssueQuestion, verb: "answer" | "dismiss", answer?: string) {
		working = true;
		failure = null;

		const path = { workspaceId: workspace.id, issueId: question.issueId, questionId: question.id };
		const outcome = await attempt({
			run: () =>
				verb === "answer"
					? api.POST("/workspaces/{workspaceId}/issues/{issueId}/questions/{questionId}/answer", {
							params: { path },
							body: { answer: answer ?? "" },
						})
					: api.POST("/workspaces/{workspaceId}/issues/{issueId}/questions/{questionId}/dismiss", {
							params: { path },
						}),
		});

		working = false;

		if (outcome.kind !== "done") {
			failure =
				outcome.kind === "refused"
					? runFailureMessage(readRunFailure(outcome.problem))
					: "Something went wrong and nothing changed. Wait a moment and try again.";

			return;
		}

		await invalidate(keys.reviews(workspace.id));
	}

	const answer = (question: IssueQuestion, given: string) => void settle(question, "answer", given);
	const dismiss = (question: IssueQuestion) => void settle(question, "dismiss");
</script>

<svelte:head>
	<title>Reviews · {workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<h1 class="text-sm text-ink-900">Reviews</h1>
			{#if queue.kind === "ready" && waitingTotal(queue.waiting) > 0}
				<span class="text-xs text-muted-foreground">{waitingCount(queue.waiting)}</span>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-4 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if queue.kind === "loading"}
				<p class="my-auto text-sm text-muted-foreground">Reading what is waiting…</p>
			{:else if queue.kind === "unavailable"}
				<div class="my-auto flex flex-col items-start gap-3">
					<Alert.Root variant="destructive">
						<CircleAlert aria-hidden="true" class="size-4" />
						<Alert.Title>We could not load the review queue</Alert.Title>
						<Alert.Description>
							Something went wrong and nothing changed. Wait a moment and try again.
						</Alert.Description>
					</Alert.Root>
					<Retry />
				</div>
			{:else if waitingTotal(queue.waiting) === 0}
				<p class="my-auto max-w-prose text-sm text-muted-foreground text-pretty">
					{noReviewsLine}
				</p>
			{:else}
				{@const planning = planningQuestions(queue.waiting)}
				{@const implementing = implementationQuestions(queue.waiting)}

				{#if failure}
					<Alert.Root variant="destructive" aria-live="polite">
						<CircleAlert aria-hidden="true" class="size-4" />
						<Alert.Title>That did not work</Alert.Title>
						<Alert.Description>{failure}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if planning.length > 0 || queue.waiting.plans.length > 0}
					<section class="flex min-w-0 flex-col gap-2" aria-label="Planning">
						<Eyebrow rule>Planning</Eyebrow>
						<div class="flex min-w-0 flex-col">
							{#each planning as item (item.question.id)}
								<ReviewQuestionRow
									{item}
									issueHref={issueHref(item.question.issueReference)}
									timezone={workspace.timezone}
									{working}
									onanswer={answer}
									ondismiss={dismiss}
								/>
							{/each}
							{#each queue.waiting.plans as item (item.run.execution.id)}
								<ReviewRunRow
									{item}
									href={workspacePath(workspace.slug, `/executions/${item.run.execution.id}#plan`)}
									timezone={workspace.timezone}
								/>
							{/each}
						</div>
					</section>
				{/if}

				{#if implementing.length > 0}
					<section class="flex min-w-0 flex-col gap-2" aria-label="Implementation">
						<Eyebrow rule>Implementation</Eyebrow>
						<div class="flex min-w-0 flex-col">
							{#each implementing as item (item.question.id)}
								<ReviewQuestionRow
									{item}
									issueHref={issueHref(item.question.issueReference)}
									timezone={workspace.timezone}
									{working}
									onanswer={answer}
									ondismiss={dismiss}
								/>
							{/each}
						</div>
					</section>
				{/if}

				{#if queue.waiting.changes.length > 0}
					<section class="flex min-w-0 flex-col gap-2" aria-label="Final review">
						<Eyebrow rule>Final review</Eyebrow>
						<div class="flex min-w-0 flex-col">
							{#each queue.waiting.changes as item (item.run.execution.id)}
								<ReviewRunRow
									{item}
									href={workspacePath(workspace.slug, `/executions/${item.run.execution.id}/review`)}
									timezone={workspace.timezone}
								/>
							{/each}
						</div>
					</section>
				{/if}
			{/if}
		</div>
	</div>
</div>
