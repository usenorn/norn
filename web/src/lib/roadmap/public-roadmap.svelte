<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { onDate } from "$lib/time";
	import RoadmapItemRow from "./roadmap-item-row.svelte";
	import { timelineWindow, type PublicRoadmap } from "./public-roadmap";

	let { roadmap, entryHref }: { roadmap: PublicRoadmap; entryHref: (id: string) => string } =
		$props();

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
			Published <time datetime={roadmap.publishedAt}>{onDate(roadmap.publishedAt, "UTC")}</time>
		</p>
	</header>

	{#if roadmap.projects.length > 0}
		<section class="flex flex-col gap-2" aria-labelledby="roadmap-projects">
			<Eyebrow id="roadmap-projects" class="text-ink-600">Projects</Eyebrow>
			<ul class="flex flex-col rounded-lg border border-line-subtle">
				{#each roadmap.projects as project (project.id)}
					<li class="border-b border-line-subtle last:border-b-0">
						<RoadmapItemRow item={project} href={entryHref(project.id)} {window} />
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	{#if roadmap.issues.length > 0}
		<section class="flex flex-col gap-2" aria-labelledby="roadmap-issues">
			<Eyebrow id="roadmap-issues" class="text-ink-600">Other planned work</Eyebrow>
			<ul class="flex flex-col rounded-lg border border-line-subtle">
				{#each roadmap.issues as issue (issue.id)}
					<li class="border-b border-line-subtle last:border-b-0">
						<RoadmapItemRow item={issue} href={entryHref(issue.id)} {window} />
					</li>
				{/each}
			</ul>
		</section>
	{/if}
</article>
