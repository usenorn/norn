<script lang="ts">
	import { navigating, page } from "$app/state";
	import PublicRoadmapFallback from "#lib/roadmap/public-roadmap-fallback.svelte";
	import PublicRoadmapFrame from "#lib/roadmap/public-roadmap-frame.svelte";
	import { isPublicRoadmapRoute } from "#lib/roadmap/host.js";
	import type { LayoutProps } from "./$types";

	let { children }: LayoutProps = $props();

	const loading = $derived(
		navigating.to !== null &&
			isPublicRoadmapRoute(navigating.to.route.id) &&
			navigating.to.url.pathname !== page.url.pathname
	);
</script>

<PublicRoadmapFrame>
	{#if loading}
		<PublicRoadmapFallback view={{ kind: "loading" }} />
	{:else}
		{@render children()}
	{/if}
</PublicRoadmapFrame>
