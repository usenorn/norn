<script lang="ts">
	import CircleHelp from "@lucide/svelte/icons/circle-help";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { onDateAndTime } from "$lib/time";
	import { standingLine, statusLine, type IssueQuestion } from "./questions";

	let {
		questions,
		timezone,
		canAnswer,
		working,
		onanswer,
		ondismiss,
	}: {
		questions: IssueQuestion[];
		timezone: string;
		canAnswer: boolean;
		working: boolean;
		onanswer: (question: IssueQuestion, answer: string) => void;
		ondismiss: (question: IssueQuestion) => void;
	} = $props();

	let answering = $state("");
	let draft = $state("");

	function open(question: IssueQuestion) {
		answering = question.id;
		draft = question.allowFreeText ? question.standing : "";
	}

	function send(question: IssueQuestion, answer: string) {
		onanswer(question, answer);
		answering = "";
		draft = "";
	}
</script>

<ul class="flex min-w-0 flex-col divide-y divide-line-subtle">
	{#each questions as question (question.id)}
		{@const open_ = question.state === "asked" && !question.expired}
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
						<span
							class="text-xs {open_ && question.blocking
								? 'text-amber-700 dark:text-amber-400'
								: 'text-muted-foreground'}"
						>
							{statusLine(question)}
						</span>
					</div>

					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{standingLine(question)}
					</p>

					<div class="flex flex-wrap items-center gap-1.5">
						{#if question.askedByName}
							<Tag name="{question.askedByName} asked" />
						{/if}
						{#if question.executionId}
							<Tag name="while running" />
						{/if}
						<span class="font-mono text-xs text-muted-foreground">
							{question.answered && question.answeredAt
								? onDateAndTime(question.answeredAt, timezone)
								: onDateAndTime(question.deadline, timezone)}
						</span>
					</div>

					{#if canAnswer && open_}
						{#if question.options && question.options.length > 0}
							<div class="flex flex-wrap gap-1.5">
								{#each question.options as option (option)}
									<Button
										variant="secondary"
										size="sm"
										disabled={working}
										onclick={() => send(question, option)}
									>
										{option}
									</Button>
								{/each}
							</div>
						{/if}

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
										onclick={() => send(question, draft)}
									>
										Answer
									</Button>
									<Button variant="ghost" size="sm" disabled={working} onclick={() => (answering = "")}>
										Cancel
									</Button>
								</div>
							</div>
						{:else}
							<div class="flex flex-wrap gap-1.5">
								{#if question.allowFreeText}
									<Button
										variant={question.options?.length ? "ghost" : "secondary"}
										size="sm"
										class="w-max"
										disabled={working}
										onclick={() => open(question)}
									>
										{question.options?.length ? "Say something else" : "Answer this"}
									</Button>
								{/if}
								<Button
									variant="ghost"
									size="sm"
									class="w-max"
									disabled={working}
									onclick={() => ondismiss(question)}
								>
									Dismiss
								</Button>
							</div>
						{/if}
					{/if}
				</div>
			</div>
		</li>
	{/each}
</ul>
