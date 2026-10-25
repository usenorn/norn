<script lang="ts">
	import { page } from "$app/state";
	import PublicRoadmapFallback from "#lib/roadmap/public-roadmap-fallback.svelte";
	import { roadmapPath } from "#lib/roadmap/public-roadmap.js";

	const notFound = 404;
</script>

{#if page.status === notFound}
	<PublicRoadmapFallback view={{ kind: "unavailable" }} roadmapHref={roadmapPath} />
{:else}
	<PublicRoadmapFallback view={{ kind: "failed", reference: page.error?.reference ?? "" }} />
{/if}
