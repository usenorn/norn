<script lang="ts">
	import CircleHelp from "@lucide/svelte/icons/circle-help";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { onDateAndTime } from "$lib/time";
	import type { IssueQuestion } from "./questions";

	let {
		questions,
		timezone,
		canAnswer,
		working,
		onanswer,
	}: {
		questions: IssueQuestion[];
		timezone: string;
		canAnswer: boolean;
		working: boolean;
		onanswer: (question: IssueQuestion, answer: string) => void;
	} = $props();

	let answering = $state("");
	let draft = $state("");

	function open(question: IssueQuestion) {
		answering = question.id;
		draft = question.standing;
	}

	function send(question: IssueQuestion) {
		onanswer(question, draft);
		answering = "";
		draft = "";
	}
</script>

<ul class="flex min-w-0 flex-col divide-y divide-line-subtle">
	{#each questions as question (question.id)}
		<li class="flex min-w-0 flex-col gap-1.5 py-2">
			<div class="flex min-w-0 items-start gap-2">
				<span class="mt-0.5 text-muted-foreground">
					<CircleHelp aria-hidden="true" class="size-3.5" />
				</span>

				<div class="flex min-w-0 flex-1 flex-col gap-1">
					<div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1">
						<span class="min-w-0 flex-1 text-sm leading-normal text-ink-900 text-pretty">
							{question.question}
						</span>
						{#if question.answered}
							<span class="text-xs text-muted-foreground">Answered</span>
						{:else if question.expired}
							<span class="text-xs text-amber-700 dark:text-amber-400">Working on the default</span>
						{:else}
							<span class="text-xs text-muted-foreground">Waiting on you</span>
						{/if}
					</div>

					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{#if question.answered}
							{question.answeredByName ?? "Someone"} answered: {question.answer}
						{:else}
							Working on the default meanwhile: {question.default}
						{/if}
					</p>

					<div class="flex flex-wrap items-center gap-1.5">
						{#if question.askedByName}
							<Tag name="{question.askedByName} asked" />
						{/if}
						<span class="font-mono text-xs text-muted-foreground">
							{question.answered && question.answeredAt
								? onDateAndTime(question.answeredAt, timezone)
								: onDateAndTime(question.deadline, timezone)}
						</span>
					</div>

					{#if canAnswer && !question.answered}
						{#if answering === question.id}
							<div class="flex min-w-0 flex-col gap-1.5">
								<Textarea
									bind:value={draft}
									rows={2}
									aria-label="Your answer"
									disabled={working}
								/>
								<div class="flex flex-wrap gap-1.5">
									<Button
										size="sm"
										disabled={working || draft.trim() === ""}
										onclick={() => send(question)}
									>
										Answer
									</Button>
									<Button variant="ghost" size="sm" disabled={working} onclick={() => (answering = "")}>
										Cancel
									</Button>
								</div>
							</div>
						{:else}
							<Button variant="secondary" size="sm" class="w-max" onclick={() => open(question)}>
								Answer this
							</Button>
						{/if}
					{/if}
				</div>
			</div>
		</li>
	{/each}
</ul>
