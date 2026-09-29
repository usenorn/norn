<script lang="ts">
	import * as Tabs from "$lib/components/ui/tabs/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Markdown from "$lib/issues/markdown.svelte";
	import { onDateAndTime } from "$lib/time";
	import DecisionActions from "./decision-actions.svelte";
	import type { Execution, IssueQuestion } from "./executions";
	import type { DecisionRight } from "./reviews";
	import {
		latestPlan,
		noPlanLine,
		planDecision,
		planFeedbackMaxLength,
		planStanding,
		revisionLabel,
		type ExecutionPlan,
		type PlanStanding,
	} from "./plans";

	let {
		execution,
		plans,
		questions,
		right,
		timezone,
		working,
		onapprove,
		onrevise,
	}: {
		execution: Execution;
		plans: ExecutionPlan[];
		questions: IssueQuestion[];
		right: DecisionRight;
		timezone: string;
		working: boolean;
		onapprove: (revision: number) => void;
		onrevise: (revision: number, feedback: string) => Promise<boolean>;
	} = $props();

	const latest = $derived(latestPlan(plans));

	let chosen = $state<string | null>(null);

	const shown = $derived(
		plans.find((plan) => String(plan.revision) === chosen) ?? latest
	);

	function standingTone(standing: PlanStanding) {
		switch (standing.kind) {
			case "approved":
				return "success";
			case "waiting":
				return "attention";
			default:
				return "muted";
		}
	}

	function standingLabel(standing: PlanStanding): string {
		switch (standing.kind) {
			case "approved":
				return `Approved by ${standing.by}`;
			case "revision_requested":
				return `${standing.by} asked for changes`;
			case "waiting":
				return "Waiting for approval";
			case "superseded":
				return "Replaced by a newer revision";
		}
	}

	function feedbackOn(plan: ExecutionPlan): ExecutionPlan | undefined {
		return plans.find((held) => held.revision === plan.revision - 1 && held.revisionFeedback);
	}
</script>

{#snippet revision(plan: ExecutionPlan)}
	{@const standing = planStanding(plan, latest)}
	{@const answering = feedbackOn(plan)}
	{@const decision = planDecision(execution, plan, latest, questions, right)}

	<div class="flex min-w-0 flex-col gap-3">
		<div class="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
			<span class="font-mono text-xs text-ink-900">{revisionLabel(plan)}</span>
			<span class="font-mono text-2xs text-muted-foreground">
				proposed {onDateAndTime(plan.proposedAt, timezone)}
			</span>
			<Eyebrow tone={standingTone(standing)}>{standingLabel(standing)}</Eyebrow>
		</div>

		{#if answering}
			<figure class="flex min-w-0 flex-col gap-1 border-l-2 border-line-strong pl-3">
				<figcaption class="text-2xs text-muted-foreground">
					This revision answers what {answering.revisionRequestedByName || "somebody"} asked of revision
					{answering.revision}
				</figcaption>
				<blockquote class="text-sm leading-normal break-words whitespace-pre-line text-ink-900">
					{answering.revisionFeedback}
				</blockquote>
			</figure>
		{/if}

		<article class="min-w-0 rounded-sm border border-line-default bg-card px-4 py-3">
			<Markdown source={plan.body} />
		</article>

		{#if standing.kind === "revision_requested"}
			<figure class="flex min-w-0 flex-col gap-1 border-l-2 border-line-strong pl-3">
				<figcaption class="text-2xs text-muted-foreground">
					{standing.by} asked for changes {onDateAndTime(standing.at, timezone)}
				</figcaption>
				<blockquote class="text-sm leading-normal break-words whitespace-pre-line text-ink-900">
					{standing.feedback}
				</blockquote>
			</figure>
		{/if}

		{#if decision.kind === "not_yours"}
			<p class="text-2xs text-muted-foreground">{decision.reason}</p>
		{:else if decision.kind !== "closed"}
			<DecisionActions
				{working}
				blocked={decision.kind === "blocked" ? decision.reason : undefined}
				approveLabel="Approve this plan"
				confirmPrompt={`Build ${revisionLabel(plan).toLowerCase()} as written?`}
				confirmLabel="Approve and build"
				requestLabel="Ask for changes"
				feedbackLabel="What should change in the plan"
				feedbackPlaceholder="Say what should change. It is handed to the coding agent word for word."
				sendLabel="Send the plan back"
				sendHint="The coding agent revises the plan in the same session. Nothing is built yet."
				feedbackMaxLength={planFeedbackMaxLength}
				onapprove={() => onapprove(plan.revision)}
				onrequest={(feedback) => onrevise(plan.revision, feedback)}
			/>
		{/if}
	</div>
{/snippet}

<section id="plan" class="flex min-w-0 scroll-mt-4 flex-col gap-3" aria-label="Plan">
	<Eyebrow rule>Plan</Eyebrow>

	{#if !shown}
		<p class="max-w-prose text-xs leading-normal text-muted-foreground">{noPlanLine(execution)}</p>
	{:else if plans.length === 1}
		{@render revision(shown)}
	{:else}
		<Tabs.Root
			value={String(shown.revision)}
			onValueChange={(value) => (chosen = value)}
			class="gap-3"
		>
			<Tabs.List variant="line" class="w-full justify-start overflow-x-auto">
				{#each plans as plan (plan.revision)}
					<Tabs.Trigger value={String(plan.revision)} class="flex-none">
						{revisionLabel(plan)}
					</Tabs.Trigger>
				{/each}
			</Tabs.List>

			{#each plans as plan (plan.revision)}
				<Tabs.Content value={String(plan.revision)}>
					{@render revision(plan)}
				</Tabs.Content>
			{/each}
		</Tabs.Root>
	{/if}
</section>
