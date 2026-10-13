<script lang="ts">
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import RoadmapItemRow from "./roadmap-item-row.svelte";
	import RoadmapStatus from "./roadmap-status.svelte";
	import {
		progressLabel,
		timelineLabel,
		timelineWindow,
		type PublicRoadmap,
		type PublicRoadmapItem,
	} from "./public-roadmap";

	let {
		roadmap,
		item,
		entryHref,
		roadmapHref,
	}: {
		roadmap: PublicRoadmap;
		item: PublicRoadmapItem;
		entryHref: (id: string) => string;
		roadmapHref: string;
	} = $props();

	const dates = $derived(timelineLabel(item.timeline));
	const progress = $derived(progressLabel(item.progress));
	const window = $derived(item.issues ? timelineWindow(item.issues) : undefined);
</script>

<article class="flex flex-col gap-8">
	<header class="flex flex-col gap-3">
		<a
			href={roadmapHref}
			class="inline-flex min-w-0 items-center gap-1.5 self-start rounded-sm text-sm text-muted-foreground hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
		>
			<ArrowLeft class="size-3.5 shrink-0" aria-hidden="true" />
			<span class="break-words">{roadmap.title}</span>
		</a>
		<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
			<h1 class="min-w-0 text-2xl font-medium tracking-title break-words text-ink-900">
				{item.title}
			</h1>
			{#if item.status}
				<RoadmapStatus status={item.status} />
			{/if}
		</div>
	</header>

	{#if dates || progress}
		<dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
			{#if dates}
				<dt class="text-muted-foreground">Timeline</dt>
				<dd class="font-mono text-ink-900 tabular-nums">{dates}</dd>
			{/if}
			{#if progress && item.progress !== undefined}
				<dt class="text-muted-foreground">Progress</dt>
				<dd class="flex items-center gap-2 font-mono text-ink-900 tabular-nums">
					<Progress value={item.progress} class="w-24" aria-label={progress} />
					<span>{progress}</span>
				</dd>
			{/if}
		</dl>
	{/if}

	{#if item.description}
		<p class="text-base leading-normal break-words whitespace-pre-line text-ink-900">
			{item.description}
		</p>
	{/if}

	{#if item.issues}
		<section class="flex flex-col gap-2" aria-labelledby="entry-issues">
			<Eyebrow id="entry-issues" class="text-ink-600">Issues</Eyebrow>
			{#if item.issues.length > 0}
				<ul class="flex flex-col rounded-lg border border-line-subtle">
					{#each item.issues as issue (issue.id)}
						<li class="border-b border-line-subtle last:border-b-0">
							<RoadmapItemRow item={issue} href={entryHref(issue.id)} {window} />
						</li>
					{/each}
				</ul>
			{:else}
				<p class="text-sm text-muted-foreground">No issues are shared for this project yet.</p>
			{/if}
		</section>
	{/if}
</article>
