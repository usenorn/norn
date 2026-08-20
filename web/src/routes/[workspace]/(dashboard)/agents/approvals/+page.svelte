<script lang="ts">
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { goto, invalidate } from "$app/navigation";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import { workspacePath } from "$lib/workspace/navigation";
	import { keys } from "$lib/api/keys";
	import { agentsPath, type ProposalQueue, type ProposedCheckEdit } from "$lib/agents/agents";
	import ProposalCard from "$lib/agents/proposal-card.svelte";
	import { approvalsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? approvalsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let deciding = $state<string | null>(null);
	let failed = $state<string | null>(null);

	const workspace = $derived(data.workspace);
	const queue = $derived<ProposalQueue>(preview?.queue ?? data.queue);
	const waiting = $derived(queue.kind === "ready" ? queue.proposals : []);
	const busy = $derived(preview?.busy || deciding !== null);

	const cursor = listCursor(() => ({
		rows: waiting,
		open: (proposal) => {
			if (!proposal.issueReference) return;

			void goto(workspacePath(workspace.slug, `/issues/${proposal.issueReference}`));
		},
	}));

	async function decide(
		proposalId: string,
		verdict: "approve" | "reject",
		checks?: ProposedCheckEdit[]
	) {
		deciding = proposalId;
		failed = null;

		try {
			const { data: decided, error } =
				verdict === "approve"
					? await api.POST("/workspaces/{workspaceId}/agent-proposals/{proposalId}/approve", {
							params: { path: { workspaceId: workspace.id, proposalId } },
							...(checks ? { body: { checks } } : {}),
						})
					: await api.POST("/workspaces/{workspaceId}/agent-proposals/{proposalId}/reject", {
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

			await invalidate(keys.page(page.route.id));
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
						<ProposalCard
							{proposal}
							slug={workspace.slug}
							timezone={workspace.timezone}
							{busy}
							cursor={cursor.holds(proposal)}
							{deciding}
							ondecide={decide}
						/>
					{/each}
				</ul>
			{/if}
		</div>
	</div>

	<ShortcutBar ids={["cursor-down", "cursor-open", "issue-new", "search", "help"]} />
</div>
