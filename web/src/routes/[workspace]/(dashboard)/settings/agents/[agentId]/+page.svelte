<script lang="ts">
	import { page } from "$app/state";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Bot from "@lucide/svelte/icons/bot";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import ActivityFeedView from "$lib/activity/activity-feed.svelte";
	import { api } from "$lib/api";
	import { onDate, onDateAndTime } from "$lib/time";
	import { agentsPath } from "$lib/agents/agents";
	import { runnersPath } from "$lib/runners/runners";
	import type { ActivityFeed } from "$lib/activity/activity";
	import { agentRecordPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? agentRecordPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let loaded = $state.raw<ActivityFeed | null>(null);
	let working = $state(false);

	const workspace = $derived(data.workspace);
	const agent = $derived(preview?.agent ?? data.agent);
	const activity = $derived<ActivityFeed>(loaded ?? preview?.activity ?? data.activity);

	async function more() {
		if (activity.kind !== "ready" || !activity.nextCursor || !agent) return;

		working = true;

		try {
			const { data: page, error } = await api.GET(
				"/workspaces/{workspaceId}/agents/{agentId}/activity",
				{
					params: {
						path: { workspaceId: workspace.id, agentId: agent.agent.id },
						query: { limit: 50, cursor: activity.nextCursor },
					},
				}
			);

			if (error || !page) return;

			loaded = {
				kind: "ready",
				events: [...activity.events, ...page.events],
				nextCursor: page.nextCursor,
			};
		} finally {
			working = false;
		}
	}

	function when(instant: string): string {
		return onDateAndTime(instant, workspace.timezone);
	}
</script>

<svelte:head>
	<title>{agent?.agent.name ?? "Agent"} · {workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-3 border-b border-line-subtle px-4">
		<a
			href={agentsPath(workspace.slug)}
			class="flex items-center gap-1.5 text-sm text-muted-foreground motion-control hover:text-ink-900"
		>
			<ArrowLeft class="size-4" aria-hidden="true" />
			<span>Agents</span>
		</a>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if !agent}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That agent is not here</Alert.Title>
					<Alert.Description>It may have been removed with its workspace.</Alert.Description>
				</Alert.Root>
			{:else}
				<section class="flex flex-col gap-2">
					<div class="flex flex-wrap items-center gap-2">
						<Bot class="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
						<h1 class="text-lg font-medium tracking-snug text-ink-900">{agent.agent.name}</h1>
						<Tag name="Agent" />
						{#if agent.agent.status === "disabled"}
							<Tag name="Disabled" />
						{/if}
					</div>
					<p class="text-sm text-muted-foreground">
						Acts for {agent.ownerName || agent.ownerEmail || "somebody who has left"} · registered
						{onDate(agent.agent.createdAt, workspace.timezone)}
						{#if agent.agent.disabledAt}
							· disabled {onDate(agent.agent.disabledAt, workspace.timezone)}
						{/if}
					</p>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Everything this agent has done, across every issue and project it could reach. The
						computers it acts on, and the folders they hold, are on the
						<a href={runnersPath(workspace.slug)} class="text-link underline-offset-2 hover:underline">
							runners screen</a>.
					</p>
				</section>

				<ActivityFeedView
					feed={activity}
					{when}
					{working}
					emptyLine="This agent has not done anything yet."
					onmore={more}
				/>
			{/if}
		</div>
	</div>
</div>
