<script lang="ts">
	import PublicRoadmapFallback from "#lib/roadmap/public-roadmap-fallback.svelte";
	import { roadmapPath } from "#lib/roadmap/public-roadmap.js";
</script>

<PublicRoadmapFallback view={{ kind: "unavailable" }} roadmapHref={roadmapPath} />
