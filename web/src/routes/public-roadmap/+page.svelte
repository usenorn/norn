<script lang="ts">
	import { page } from "$app/state";
	import PublicRoadmap from "$lib/roadmap/public-roadmap.svelte";
	import PublicRoadmapFallback from "$lib/roadmap/public-roadmap-fallback.svelte";
	import PublicRoadmapHead from "$lib/roadmap/public-roadmap-head.svelte";
	import { entryPath } from "$lib/roadmap/public-roadmap";
	import { publicRoadmapPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const view = $derived(
		(import.meta.env.DEV && publicRoadmapPreviewStates[page.url.searchParams.get("state") ?? ""]) ||
			data.view
	);
</script>

{#if view.kind === "ready" || view.kind === "empty"}
	<PublicRoadmapHead title={view.roadmap.title} description={view.roadmap.description} />
	<PublicRoadmap roadmap={view.roadmap} stamp="published" entryHref={entryPath} />
{:else}
	<PublicRoadmapFallback {view} />
{/if}
