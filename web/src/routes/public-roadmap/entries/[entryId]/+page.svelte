<script lang="ts">
	import { page } from "$app/state";
	import PublicRoadmapEntry from "#lib/roadmap/public-roadmap-entry.svelte";
	import PublicRoadmapFallback from "#lib/roadmap/public-roadmap-fallback.svelte";
	import PublicRoadmapHead from "#lib/roadmap/public-roadmap-head.svelte";
	import { entryPath, roadmapPath } from "#lib/roadmap/public-roadmap.js";
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
		title="{view.item.title} · {view.roadmapTitle}"
		description={view.item.description}
	/>
	<PublicRoadmapEntry
		roadmapTitle={view.roadmapTitle}
		item={view.item}
		entryHref={entryPath}
		roadmapHref={roadmapPath}
	/>
{:else}
	<PublicRoadmapFallback {view} roadmapHref={roadmapPath} />
{/if}
