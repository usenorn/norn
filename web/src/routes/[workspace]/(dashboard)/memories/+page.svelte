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
	import MemoryRow from "#lib/memories/memory-row.svelte";
	import { memoriesPath, memoryFilterLabels, memoryFilters, type MemoryFilter } from "#lib/memories/memories.js";
	import { memoriesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? memoriesPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const list = $derived(preview?.list ?? data.list);

	const emptyLines: Record<MemoryFilter, { title: string; description: string }> = {
		review: {
			title: "Nothing waiting for review",
			description:
				"A memory that repeats, qualifies, or contradicts one already recorded waits here until somebody settles it.",
		},
		accepted: {
			title: "No accepted memories yet",
			description: "Decisions, constraints, lessons, and preferences that agents and people record show up here.",
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
		<nav aria-label="Filter memories" class="flex">
			<div class="scrollbar-none flex min-w-0 gap-0.5 overflow-x-auto rounded-sm border border-line-default p-0.5">
				{#each memoryFilters as filter (filter)}
					<SegmentLink href={memoriesPath(slug, filter)} current={data.filter === filter}>
						{memoryFilterLabels[filter]}
					</SegmentLink>
				{/each}
			</div>
		</nav>

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
		{:else if list.kind === "empty"}
			<Empty.Root>
				<Empty.Media variant="icon"><Brain aria-hidden="true" /></Empty.Media>
				<Empty.Header>
					<Empty.Title>{emptyLines[data.filter].title}</Empty.Title>
					<Empty.Description>{emptyLines[data.filter].description}</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<RowGroup as="ul">
				{#each list.memories as memory (memory.id)}
					<li><MemoryRow {memory} workspace={slug} projects={data.projects} /></li>
				{/each}
			</RowGroup>
			{#if list.nextCursor}
				<div>
					<Button variant="secondary" size="sm" href={memoriesPath(slug, data.filter, list.nextCursor)}>
						Older memories
					</Button>
				</div>
			{/if}
		{/if}
	</div>
</div>
