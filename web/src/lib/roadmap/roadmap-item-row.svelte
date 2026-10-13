<script lang="ts">
	import { Progress } from "$lib/components/ui/progress/index.js";
	import RoadmapStatus from "./roadmap-status.svelte";
	import RoadmapTimeline from "./roadmap-timeline.svelte";
	import {
		issueCountLabel,
		progressLabel,
		timelineLabel,
		type PublicRoadmapItem,
		type TimelineWindow,
	} from "./public-roadmap";

	let {
		item,
		href,
		window,
	}: { item: PublicRoadmapItem; href: string; window?: TimelineWindow } = $props();

	const dates = $derived(timelineLabel(item.timeline));
	const progress = $derived(progressLabel(item.progress));
	const issues = $derived(issueCountLabel(item));
</script>

<a
	{href}
	class="flex flex-col gap-2 px-3 py-3 motion-control hover:bg-paper-2 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
>
	<span class="flex flex-wrap items-center gap-x-3 gap-y-1">
		<span class="min-w-0 flex-[1_1_160px] text-md break-words text-ink-900">{item.title}</span>
		{#if item.status}
			<RoadmapStatus status={item.status} />
		{/if}
	</span>

	{#if item.description}
		<span class="line-clamp-2 text-sm leading-normal break-words whitespace-pre-line text-muted-foreground">
			{item.description}
		</span>
	{/if}

	{#if dates || progress || issues}
		<span class="flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-xs text-muted-foreground tabular-nums">
			{#if dates}
				<span>{dates}</span>
			{/if}
			{#if issues}
				<span>{issues}</span>
			{/if}
			{#if progress && item.progress !== undefined}
				<span class="flex items-center gap-2">
					<Progress value={item.progress} class="w-16" aria-label={progress} />
					<span>{progress}</span>
				</span>
			{/if}
		</span>
	{/if}

	{#if window}
		<RoadmapTimeline timeline={item.timeline} {window} />
	{/if}
</a>
