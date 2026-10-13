<script lang="ts">
	import Map from "@lucide/svelte/icons/map";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import { onDate } from "$lib/time";
	import RoadmapItemList from "./roadmap-item-list.svelte";
	import { timelineWindow, type PublicRoadmap, type RoadmapStamp } from "./public-roadmap";

	let {
		roadmap,
		stamp,
		entryHref,
	}: { roadmap: PublicRoadmap; stamp: RoadmapStamp; entryHref: (id: string) => string } = $props();

	const window = $derived(timelineWindow([...roadmap.projects, ...roadmap.issues]));
</script>

<article class="flex flex-col gap-8">
	<header class="flex flex-col gap-2">
		<h1 class="text-2xl font-medium tracking-title break-words text-ink-900">{roadmap.title}</h1>
		{#if roadmap.description}
			<p class="text-base leading-normal break-words whitespace-pre-line text-muted-foreground">
				{roadmap.description}
			</p>
		{/if}
		<p class="font-mono text-xs text-muted-foreground">
			{#if stamp === "published"}
				Published <time datetime={roadmap.publishedAt}>{onDate(roadmap.publishedAt, "UTC")}</time>
			{:else}
				Draft · not published yet
			{/if}
		</p>
	</header>

	{#if roadmap.projects.length === 0 && roadmap.issues.length === 0}
		<Empty.Root>
			<Empty.Media variant="icon"><Map aria-hidden="true" /></Empty.Media>
			<Empty.Header>
				<Empty.Title>Nothing on this roadmap yet</Empty.Title>
				<Empty.Description>Planned work will appear here once it is shared.</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	{/if}

	{#if roadmap.projects.length > 0}
		<section class="flex flex-col gap-2" aria-labelledby="roadmap-projects">
			<h2 id="roadmap-projects"><Eyebrow class="text-ink-600">Projects</Eyebrow></h2>
			<RoadmapItemList items={roadmap.projects} {entryHref} {window} />
		</section>
	{/if}

	{#if roadmap.issues.length > 0}
		<section class="flex flex-col gap-2" aria-labelledby="roadmap-issues">
			<h2 id="roadmap-issues"><Eyebrow class="text-ink-600">Other planned work</Eyebrow></h2>
			<RoadmapItemList items={roadmap.issues} {entryHref} {window} />
		</section>
	{/if}
</article>
