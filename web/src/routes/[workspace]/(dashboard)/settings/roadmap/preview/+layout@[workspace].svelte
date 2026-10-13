<script lang="ts">
	import { navigating, page } from "$app/state";
	import { roadmapPreviewPath } from "$lib/roadmap/admin";
	import PublicRoadmapFrame from "$lib/roadmap/public-roadmap-frame.svelte";
	import RoadmapPreviewBar from "$lib/roadmap/roadmap-preview-bar.svelte";
	import RoadmapPreviewFallback from "$lib/roadmap/roadmap-preview-fallback.svelte";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const previewRoute = "/[workspace]/(dashboard)/settings/roadmap/preview";

	const loading = $derived(
		navigating.to !== null &&
			(navigating.to.route.id?.startsWith(previewRoute) ?? false) &&
			navigating.to.url.pathname !== page.url.pathname
	);
</script>

<PublicRoadmapFrame>
	{#snippet banner()}
		<RoadmapPreviewBar workspace={data.workspace.slug} />
	{/snippet}

	{#if loading}
		<RoadmapPreviewFallback
			view={{ kind: "loading" }}
			workspaceName={data.workspace.name}
			previewHref={roadmapPreviewPath(data.workspace.slug)}
		/>
	{:else}
		{@render children()}
	{/if}
</PublicRoadmapFrame>
