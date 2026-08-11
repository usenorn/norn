<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
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
		ondecide: (proposalId: string, verdict: "approve" | "reject") => void;
	} = $props();

	const checkSet = $derived(proposal.action === "check_set");
	const override = $derived(overriding(proposal));
	const proposed = $derived(proposal.proposedChecks ?? []);
	const held = $derived(proposal.checkState);
	const settled = $derived(deciding === proposal.id);
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
			<Eyebrow class="text-warning">Criteria it wants to add</Eyebrow>
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
		<Button size="sm" disabled={busy} onclick={() => ondecide(proposal.id, "approve")}>
			{#if settled}
				Applying…
			{:else if override}
				Approve anyway
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
