<script lang="ts">
	import Brain from "@lucide/svelte/icons/brain";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { page } from "$app/state";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Empty from "#lib/components/ui/empty/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import RowGroup from "#lib/components/norn/row-group.svelte";
	import SegmentLink from "#lib/components/norn/segment-link.svelte";
	import MemoriesHeader from "#lib/memories/memories-header.svelte";
	import MemoryFilterBar from "#lib/memories/memory-filter-bar.svelte";
	import MemoryReviewActions from "#lib/memories/memory-review-actions.svelte";
	import MemoryRow from "#lib/memories/memory-row.svelte";
	import RejectMemoryDialog from "#lib/memories/reject-memory-dialog.svelte";
	import {
		memoriesPath,
		memoryPending,
		memoryQueryFiltered,
		memoryTabLabels,
		memoryTabs,
		type Memory,
		type MemoryTab,
	} from "#lib/memories/memories.js";
	import { memoriesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? memoriesPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const list = $derived(preview?.list ?? data.list);
	const query = $derived(preview?.query ?? data.query);
	const filtered = $derived(memoryQueryFiltered(query));
	const components = $derived(
		list.kind === "ready" ? list.memories.flatMap((memory) => memory.appliesTo.component ?? []) : []
	);

	let rejecting = $state<Memory | null>(null);
	let rejectOpen = $state(false);

	function reject(memory: Memory) {
		rejecting = memory;
		rejectOpen = true;
	}

	const emptyLines: Record<MemoryTab, { title: string; description: string }> = {
		accepted: {
			title: "No accepted memories yet",
			description:
				"Decisions, constraints, lessons, and preferences that agents recall show up here once they are accepted.",
		},
		review: {
			title: "Nothing waits for your decision",
			description:
				"Workspace admins decide on workspace memories, and project leads on their project's. A proposal they can settle waits here.",
		},
		all: {
			title: "No memories yet",
			description: "Decisions, constraints, lessons, and preferences that agents and people record show up here.",
		},
	};
</script>

<svelte:head>
	<title>Memories · {data.workspace.name} · Norn</title>
</svelte:head>

<MemoriesHeader workspace={slug} />

<div class="flex-1 overflow-y-auto">
	<div
		class="mx-auto flex w-full max-w-180 flex-col gap-4 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
	>
		<nav aria-label="Memories" class="flex">
			<div class="scrollbar-none flex min-w-0 gap-0.5 overflow-x-auto rounded-sm border border-line-default p-0.5">
				{#each memoryTabs as tab (tab)}
					<SegmentLink href={memoriesPath(slug, { ...query, tab })} current={query.tab === tab}>
						{memoryTabLabels[tab]}
					</SegmentLink>
				{/each}
			</div>
		</nav>

		<MemoryFilterBar workspace={slug} {query} projects={data.projects} {components} />

		{#if list.kind === "loading"}
			<div class="flex flex-col gap-1.5" aria-busy="true" aria-label="Loading memories">
				{#each [0, 1, 2, 3] as row (row)}
					<div class="h-11 animate-breathe rounded-xs bg-paper-2"></div>
				{/each}
			</div>
		{:else if list.kind === "unavailable"}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not load the memories</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		{:else if list.kind === "empty" && filtered}
			<Empty.Root>
				<Empty.Media variant="icon"><Brain aria-hidden="true" /></Empty.Media>
				<Empty.Header>
					<Empty.Title>No memories match these filters</Empty.Title>
					<Empty.Description>Loosen a filter, or clear them to see every memory here.</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button variant="secondary" size="sm" href={memoriesPath(slug, { tab: query.tab })}>Clear filters</Button>
				</Empty.Content>
			</Empty.Root>
		{:else if list.kind === "empty"}
			<Empty.Root>
				<Empty.Media variant="icon"><Brain aria-hidden="true" /></Empty.Media>
				<Empty.Header>
					<Empty.Title>{emptyLines[query.tab].title}</Empty.Title>
					<Empty.Description>{emptyLines[query.tab].description}</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<RowGroup as="ul">
				{#each list.memories as memory (memory.id)}
					<li>
						{#if query.tab === "review" && memory.viewer.canReview && memoryPending(memory)}
							<MemoryRow {memory} workspace={slug} projects={data.projects}>
								{#snippet actions()}
									<MemoryReviewActions {memory} workspaceId={data.workspace.id} onreject={reject} />
								{/snippet}
							</MemoryRow>
						{:else}
							<MemoryRow {memory} workspace={slug} projects={data.projects} />
						{/if}
					</li>
				{/each}
			</RowGroup>
			{#if list.nextCursor}
				<div>
					<Button variant="secondary" size="sm" href={memoriesPath(slug, query, list.nextCursor)}>
						Older memories
					</Button>
				</div>
			{/if}
		{/if}
	</div>
</div>

{#if rejecting}
	<RejectMemoryDialog bind:open={rejectOpen} memory={rejecting} workspaceId={data.workspace.id} />
{/if}
