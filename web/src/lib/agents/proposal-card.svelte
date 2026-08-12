<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { onDateAndTime } from "$lib/time";
	import { workspacePath } from "$lib/workspace/navigation";
	import AgentReasoning from "./agent-reasoning.svelte";
	import {
		actionLabels,
		blockingLine,
		checkStateLabels,
		checkStateTones,
		methodLabels,
		overriding,
		proposalSummary,
		type AgentProposal,
		type ProposedCheckEdit,
	} from "./agents";

	let {
		proposal,
		slug,
		timezone,
		busy,
		deciding,
		ondecide,
	}: {
		proposal: AgentProposal;
		slug: string;
		timezone: string;
		busy: boolean;
		deciding: string | null;
		ondecide: (
			proposalId: string,
			verdict: "approve" | "reject",
			checks?: ProposedCheckEdit[]
		) => void;
	} = $props();

	const checkSet = $derived(proposal.action === "check_set");
	const override = $derived(overriding(proposal));
	const proposed = $derived(proposal.proposedChecks ?? []);
	const held = $derived(proposal.checkState);
	const settled = $derived(deciding === proposal.id);
	const questions = $derived(proposal.questions ?? []);

	let edits = $state<ProposedCheckEdit[] | null>(null);

	function correct() {
		edits = proposed.map((check) => ({
			id: check.id,
			statement: check.statement,
			method: check.method,
			proof: check.proof,
			timeLimitSeconds: check.timeLimitSeconds,
		}));
	}

	function drop(index: number) {
		edits = (edits ?? []).filter((_, at) => at !== index);
	}

	function add() {
		edits = [
			...(edits ?? []),
			{ statement: "", method: "command", proof: "" },
		];
	}

	const incomplete = $derived(
		(edits ?? []).some((edit) => edit.statement.trim() === "" || edit.proof.trim() === "")
	);
</script>

<li class="flex flex-col gap-3 rounded-lg border border-line-subtle bg-paper-0 p-4">
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

	{#if checkSet && proposed.length > 0}
		<div class="flex flex-col gap-1.5 rounded-md border border-warning/40 p-2.5">
			<div class="flex flex-wrap items-baseline justify-between gap-2">
				<Eyebrow class="text-warning">Criteria it wants to add</Eyebrow>
				{#if edits === null}
					<Button variant="ghost" size="sm" disabled={busy} onclick={correct}>Correct these</Button>
				{/if}
			</div>

			{#if edits === null}
				<ol class="flex flex-col gap-1.5">
					{#each proposed as check (check.id)}
						<li class="flex flex-col gap-0.5">
							<span class="text-sm leading-normal text-ink-900 text-pretty">{check.statement}</span>
							<span class="text-xs text-muted-foreground text-pretty">
								<span class="font-mono tracking-eyebrow uppercase">
									{methodLabels[check.method]}
								</span>
								· {check.proof}
							</span>
						</li>
					{/each}
				</ol>
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					A check set always waits for a person, whatever this team's other settings say. A new
					criterion changes what done means here.
				</p>
			{:else}
				<ol class="flex flex-col gap-2.5">
					{#each edits as edit, index (index)}
						<li class="flex flex-col gap-1.5 border-t border-line-subtle pt-2.5 first:border-t-0 first:pt-0">
							<Input
								bind:value={edit.statement}
								aria-label="What must be true"
								placeholder="What must be true"
								disabled={busy}
							/>
							<Textarea
								bind:value={edit.proof}
								rows={2}
								aria-label="How it is proven"
								placeholder="How it is proven"
								disabled={busy}
							/>
							<Button
								variant="ghost"
								size="sm"
								class="w-max"
								disabled={busy}
								onclick={() => drop(index)}
							>
								Drop this one
							</Button>
						</li>
					{/each}
				</ol>
				<div class="flex flex-wrap gap-2">
					<Button variant="secondary" size="sm" disabled={busy} onclick={add}>
						Add a criterion
					</Button>
					<Button variant="ghost" size="sm" disabled={busy} onclick={() => (edits = null)}>
						Leave them as proposed
					</Button>
				</div>
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					What you approve is what the issue is judged on, and the edits are recorded as yours.
				</p>
			{/if}
		</div>
	{/if}

	{#if held && !checkSet}
		<div class="flex flex-col gap-1.5 rounded-md border border-line-subtle p-2.5">
			<div class="flex flex-wrap items-baseline justify-between gap-2">
				<Eyebrow class="text-ink-600">
					What done means on {proposal.issueReference ?? "this issue"}
				</Eyebrow>
				<span class="text-xs text-muted-foreground">
					{held.summary.proven}/{held.summary.total} proven
				</span>
			</div>

			{#if held.summary.total === 0}
				<p class="text-xs leading-normal text-muted-foreground text-pretty">
					This issue carries no criteria, so nothing here says whether it is finished.
				</p>
			{:else}
				<ul class="flex flex-col gap-0.5">
					{#each held.checks as check (check.id)}
						<li class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
							<span class="min-w-0 flex-1 text-sm leading-normal text-ink-900 text-pretty">
								{check.statement}
							</span>
							<span class="text-xs {checkStateTones[check.state ?? 'unproven']}">
								{checkStateLabels[check.state ?? "unproven"]}
							</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}

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

	{#if override && held}
		<Alert.Root variant="warning">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>This finishes work that is not proven</Alert.Title>
			<Alert.Description>
				{blockingLine(held.summary)} Approving applies the move as {proposal.agentName} and records
				that you let it past.
			</Alert.Description>
		</Alert.Root>
	{/if}

	<div class="flex flex-wrap gap-2">
		<Button
			size="sm"
			disabled={busy || incomplete}
			onclick={() => ondecide(proposal.id, "approve", edits ?? undefined)}
		>
			{#if settled}
				Applying…
			{:else if override}
				Approve anyway
			{:else if edits !== null}
				Approve as corrected
			{:else if checkSet}
				Approve these criteria
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
