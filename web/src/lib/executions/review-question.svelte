<script lang="ts">
	import QuestionList from "$lib/questions/question-list.svelte";
	import type { IssueQuestion } from "$lib/questions/questions";
	import { waitingOnLine, type ReviewQuestion } from "./reviews";

	let {
		item,
		issueHref,
		timezone,
		working,
		onanswer,
		ondismiss,
	}: {
		item: ReviewQuestion;
		issueHref: string;
		timezone: string;
		working: boolean;
		onanswer: (question: IssueQuestion, answer: string) => void;
		ondismiss: (question: IssueQuestion) => void;
	} = $props();
</script>

<div class="flex min-w-0 flex-col border-b border-line-subtle px-1 pt-2 last:border-b-0">
	<a href={issueHref} class="flex min-w-0 items-baseline gap-2.5 hover:underline">
		<span class="w-20 flex-none font-mono text-xs text-muted-foreground sm:w-24">
			{item.question.issueReference}
		</span>
		<span class="truncate text-xs text-ink-900">{item.question.issueTitle}</span>
	</a>
	<QuestionList
		questions={[item.question]}
		{timezone}
		canAnswer={item.decision.canDecide}
		refusal={waitingOnLine(item.decision)}
		{working}
		{onanswer}
		{ondismiss}
	/>
</div>
