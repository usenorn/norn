<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { onDateAndTime } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import AgentReasoning from "./agent-reasoning.svelte";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import Markdown from "$lib/issues/markdown.svelte";
	import { priorityLabel } from "$lib/issues/issues";
	import {
		actionLabels,
		changePartLabels,
		proposalSummary,
		type AgentProposal,
		type ChangePart,
	} from "./agents";

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
		ondecide: (
			proposalId: string,
			verdict: "approve" | "reject",
			accept?: ChangePart[]
		) => void;
	} = $props();

	const id = $props.id();
	const settled = $derived(deciding === proposal.id);
	const questions = $derived(proposal.questions ?? []);
	const parts = $derived(proposal.parts ?? []);
	const held = $derived(proposal.held);

	// Every part starts accepted, because approving all of it is what approving used to mean.
	// What a reader does here is take things out, not put them in.
	let refused = $state.raw<ChangePart[]>([]);

	const accepted = $derived(parts.filter((part) => !refused.includes(part)));

	function keep(part: ChangePart, on: boolean) {
		refused = on ? refused.filter((held) => held !== part) : [...refused, part];
	}

	const cleared = $derived(proposal.cleared ?? []);

	function proposed(part: ChangePart): string {
		if (cleared.includes(part)) return "left empty";

		switch (part) {
			case "title":
				return proposal.title ?? "";
			case "description":
				return proposal.description ?? "";
			case "state":
				return proposal.stateName ?? "";
			case "priority":
				return proposal.priority ? priorityLabel(proposal.priority) : "";
			case "estimate":
				return proposal.estimate === undefined ? "" : `${proposal.estimate} points`;
			case "dueOn":
				return proposal.dueOn ?? "";
			default:
				return "a different one";
		}
	}

	function standing(part: ChangePart): string {
		switch (part) {
			case "title":
				return held?.title ?? "";
			case "description":
				return held?.description ?? "";
			case "state":
				return held?.stateName ?? "";
			case "priority":
				return held?.priority ? priorityLabel(held.priority) : "";
			default:
				return "";
		}
	}

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

	{#if parts.length > 0}
		<div class="flex flex-col gap-2">
			<Eyebrow class="text-ink-600">What it would change</Eyebrow>
			<ul class="flex flex-col gap-2">
				{#each parts as part (part)}
					<li class="flex flex-col gap-1 rounded-md border border-line-subtle p-2.5">
						<div class="flex items-center gap-1.5">
							<Checkbox
								id="{id}-{part}"
								checked={!refused.includes(part)}
								disabled={busy}
								onCheckedChange={(on) => keep(part, on === true)}
							/>
							<Label for="{id}-{part}" class="font-normal">{changePartLabels[part]}</Label>
						</div>

						{#if part === "description"}
							<div class="grid gap-2 sm:grid-cols-2">
								<div class="flex min-w-0 flex-col gap-0.5">
									<Eyebrow class="text-ink-600">Now</Eyebrow>
									<div class="max-h-48 min-w-0 overflow-y-auto rounded-sm bg-paper-2 p-2">
										{#if standing(part)}
											<Markdown source={standing(part)} />
										{:else}
											<span class="text-sm text-muted-foreground">No description.</span>
										{/if}
									</div>
								</div>
								<div class="flex min-w-0 flex-col gap-0.5">
									<Eyebrow class="text-ink-600">Proposed</Eyebrow>
									<div class="max-h-48 min-w-0 overflow-y-auto rounded-sm bg-paper-2 p-2">
										<Markdown source={proposed(part)} />
									</div>
								</div>
							</div>
						{:else if proposed(part)}
							<p class="text-sm leading-normal text-ink-900 text-pretty">
								{#if standing(part)}
									<span class="text-muted-foreground line-through">{standing(part)}</span>
									<span aria-hidden="true">→</span>
								{/if}
								{proposed(part)}
							</p>
						{/if}
					</li>
				{/each}
			</ul>
		</div>
	{/if}

	<div class="flex flex-wrap gap-2">
		<Button
			size="sm"
			disabled={busy || (parts.length > 0 && accepted.length === 0)}
			onclick={() =>
				ondecide(proposal.id, "approve", parts.length > 0 ? accepted : undefined)}
		>
			{#if settled}
				Applying…
			{:else if refused.length > 0}
				Approve {accepted.length} of {parts.length}
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
