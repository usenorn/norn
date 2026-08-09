<script lang="ts">
	import { page } from "$app/state";
	import Bot from "@lucide/svelte/icons/bot";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import { agentsPath, actionLabels, proposalSummary, type ProposalQueue } from "$lib/agents/agents";
	import { approvalsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? approvalsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let settled = $state<ProposalQueue | null>(null);
	let deciding = $state<string | null>(null);
	let failed = $state<string | null>(null);

	const workspace = $derived(data.workspace);
	const queue = $derived<ProposalQueue>(settled ?? preview?.queue ?? data.queue);
	const waiting = $derived(queue.kind === "ready" ? queue.proposals : []);
	const busy = $derived(preview?.busy || deciding !== null);

	async function decide(proposalId: string, verdict: "approve" | "reject") {
		deciding = proposalId;
		failed = null;

		try {
			const path =
				verdict === "approve"
					? ("/workspaces/{workspaceId}/agent-proposals/{proposalId}/approve" as const)
					: ("/workspaces/{workspaceId}/agent-proposals/{proposalId}/reject" as const);

			const { data: decided, error } = await api.POST(path, {
				params: { path: { workspaceId: workspace.id, proposalId } },
			});

			if (error) {
				failed = "That could not be decided. Reload and try again.";

				return;
			}

			if (decided?.status === "failed") {
				failed =
					decided.failure ||
					"The issue has moved on since the agent asked, so the change no longer applies.";
			}

			const remaining = waiting.filter((proposal) => proposal.id !== proposalId);

			settled = remaining.length === 0 ? { kind: "empty" } : { kind: "ready", proposals: remaining };
		} catch {
			failed = "We could not reach the server. Try again in a moment.";
		} finally {
			deciding = null;
		}
	}
</script>

<svelte:head><title>Waiting for approval · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center justify-between gap-3 border-b border-line-subtle px-4">
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Waiting for approval</h1>
		<a
			href={agentsPath(workspace.slug)}
			class="text-xs text-muted-foreground motion-control hover:text-ink-900"
		>
			Agents
		</a>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-160 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				These teams hold some of what an agent does until a person agrees. Approving applies the
				change as the agent, so the record still says which one did it.
			</p>

			{#if failed}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not apply</Alert.Title>
					<Alert.Description>{failed}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if queue.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not approve agent actions here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if queue.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load what is waiting</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if queue.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading…</p>
			{:else if waiting.length === 0}
				<p class="text-sm text-muted-foreground">Nothing is waiting.</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each waiting as proposal (proposal.id)}
						<li class="flex flex-col gap-3 rounded-lg border border-line-subtle bg-paper-0 p-4">
							<div class="flex flex-col gap-1.5">
								<div class="flex flex-wrap items-center gap-2">
									<Bot class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
									<span class="text-sm font-medium text-ink-900">{proposal.agentName}</span>
									<Tag name="Agent" />
									<span class="text-xs text-muted-foreground">
										{onDateAndTime(proposal.createdAt, workspace.timezone)}
									</span>
								</div>
								<p class="text-sm leading-normal text-ink-600 text-pretty">
									{actionLabels[proposal.action]} — {proposalSummary(proposal)}
								</p>
								<a
									href={`/${workspace.slug}/issues`}
									class="text-xs text-muted-foreground underline-offset-2 hover:underline"
								>
									On an issue in this workspace
								</a>
							</div>

							<div class="flex flex-wrap gap-2">
								<Button
									size="sm"
									disabled={busy}
									onclick={() => decide(proposal.id, "approve")}
								>
									{deciding === proposal.id ? "Applying…" : "Approve"}
								</Button>
								<Button
									variant="secondary"
									size="sm"
									disabled={busy}
									onclick={() => decide(proposal.id, "reject")}
								>
									Reject
								</Button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>
</div>
