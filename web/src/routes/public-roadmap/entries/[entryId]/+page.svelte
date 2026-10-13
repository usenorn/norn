<script lang="ts">
	import { page } from "$app/state";
	import PublicRoadmapEntry from "$lib/roadmap/public-roadmap-entry.svelte";
	import PublicRoadmapFallback from "$lib/roadmap/public-roadmap-fallback.svelte";
	import PublicRoadmapHead from "$lib/roadmap/public-roadmap-head.svelte";
	import { entryPath, roadmapPath } from "$lib/roadmap/public-roadmap";
	import { publicRoadmapEntryPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const view = $derived(
		(import.meta.env.DEV &&
			publicRoadmapEntryPreviewStates[page.url.searchParams.get("state") ?? ""]) ||
			data.view
	);
</script>

{#if view.kind === "ready"}
	<PublicRoadmapHead
		title="{view.item.title} · {view.roadmap.title}"
		description={view.item.description}
	/>
	<PublicRoadmapEntry
		roadmap={view.roadmap}
		item={view.item}
		entryHref={entryPath}
		roadmapHref={roadmapPath}
	/>
{:else}
	<PublicRoadmapFallback {view} roadmapHref={roadmapPath} />
{/if}
