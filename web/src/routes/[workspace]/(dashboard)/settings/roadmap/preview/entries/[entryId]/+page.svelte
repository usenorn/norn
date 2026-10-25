<script lang="ts">
	import { page } from "$app/state";
	import {
		roadmapPreviewEntryPath,
		roadmapPreviewPath,
		type RoadmapPreviewEntryView,
	} from "#lib/roadmap/admin.js";
	import PublicRoadmapEntry from "#lib/roadmap/public-roadmap-entry.svelte";
	import RoadmapPreviewFallback from "#lib/roadmap/roadmap-preview-fallback.svelte";
	import { roadmapPreviewEntryStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const view = $derived<RoadmapPreviewEntryView>(
		(import.meta.env.DEV && roadmapPreviewEntryStates[page.url.searchParams.get("state") ?? ""]) || data.view
	);
	const workspace = $derived(data.workspace);
	const previewHref = $derived(roadmapPreviewPath(workspace.slug));
</script>

<svelte:head>
	<title>
		{view.kind === "ready" ? `${view.item.title} · ` : ""}Draft preview · Public roadmap · {workspace.name} · Norn
	</title>
</svelte:head>

{#if view.kind === "ready"}
	<PublicRoadmapEntry
		roadmapTitle={view.roadmapTitle || "Draft preview"}
		item={view.item}
		entryHref={(id) => roadmapPreviewEntryPath(workspace.slug, id)}
		roadmapHref={previewHref}
	/>
{:else}
	<RoadmapPreviewFallback {view} workspaceName={workspace.name} {previewHref} />
{/if}
