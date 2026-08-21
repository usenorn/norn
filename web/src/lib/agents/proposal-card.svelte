<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { onDateAndTime } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import AgentReasoning from "./agent-reasoning.svelte";
	import { actionLabels, proposalSummary, type AgentProposal } from "./agents";

	let {
		proposal,
		slug,
		timezone,
		busy,
		cursor = false,
		deciding,
		ondecide,
	}: {
		proposal: AgentProposal;
		slug: string;
		timezone: string;
		busy: boolean;
		cursor?: boolean;
		deciding: string | null;
		ondecide: (proposalId: string, verdict: "approve" | "reject") => void;
	} = $props();

	const settled = $derived(deciding === proposal.id);
	const questions = $derived(proposal.questions ?? []);

</script>

<li
	data-cursor={cursor}
	aria-current={cursor ? "true" : undefined}
	class="cursor-row flex flex-col gap-3 rounded-lg border border-line-subtle bg-paper-0 p-4"
>
	<div class="flex flex-col gap-1.5">
		<div class="flex flex-wrap items-center gap-2">
			<Bot class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
			<span class="text-sm font-medium text-ink-900">{proposal.agentName}</span>
			<Tag name="Agent" />
			<span class="text-xs text-muted-foreground">
				{onDateAndTime(proposal.createdAt, timezone)}
			</span>
		</div>

		<p class="text-sm leading-normal text-ink-600 text-pretty">
			{actionLabels[proposal.action]} — {proposalSummary(proposal)}
		</p>

		{#if proposal.issueReference}
			<a
				href={workspacePath(slug, `/issues/${proposal.issueReference}`)}
				class="min-w-0 text-xs text-muted-foreground underline-offset-2 motion-control hover:text-ink-900 hover:underline"
			>
				<span class="font-mono">{proposal.issueReference}</span>
				{#if proposal.issueTitle}
					· {proposal.issueTitle}
				{/if}
			</a>
		{/if}
	</div>

	<AgentReasoning reasoning={proposal.reasoning} />

	{#if questions.length > 0}
		<div class="flex flex-col gap-1.5 rounded-md border border-warning/40 p-2.5">
			<Eyebrow class="text-warning">Asked, never answered</Eyebrow>
			<ol class="flex flex-col gap-1.5">
				{#each questions as question (question.id)}
					<li class="flex flex-col gap-0.5">
						<span class="text-sm leading-normal text-ink-900 text-pretty">{question.question}</span>
						<span class="text-xs text-muted-foreground text-pretty">
							Worked on: {question.default}
						</span>
					</li>
				{/each}
			</ol>
			<p class="text-xs leading-normal text-muted-foreground text-pretty">
				Approving this also ratifies {questions.length === 1 ? "that default" : "those defaults"}, and
				Norn records that you did.
			</p>
		</div>
	{/if}

	<div class="flex flex-wrap gap-2">
		<Button size="sm" disabled={busy} onclick={() => ondecide(proposal.id, "approve")}>
			{#if settled}
				Applying…
			{:else}
				Approve
			{/if}
		</Button>
		<Button
			variant="secondary"
			size="sm"
			disabled={busy}
			onclick={() => ondecide(proposal.id, "reject")}
		>
			Reject
		</Button>
	</div>
</li>
