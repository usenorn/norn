<script lang="ts">
	import { page } from "$app/state";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import {
		blockerMessage,
		roadmapPreviewEntryPath,
		roadmapPreviewPath,
		roadmapSettingsPath,
		type RoadmapPreviewView,
	} from "#lib/roadmap/admin.js";
	import PublicRoadmap from "#lib/roadmap/public-roadmap.svelte";
	import RoadmapPreviewFallback from "#lib/roadmap/roadmap-preview-fallback.svelte";
	import { roadmapPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const view = $derived<RoadmapPreviewView>(
		(import.meta.env.DEV && roadmapPreviewStates[page.url.searchParams.get("state") ?? ""]) || data.view
	);
	const workspace = $derived(data.workspace);
</script>

<svelte:head><title>Draft preview · Public roadmap · {workspace.name} · Norn</title></svelte:head>

{#if view.kind === "ready"}
	<div class="flex flex-col gap-8">
		{#if view.preview.blockers.length > 0}
			<Alert.Root variant="warning">
				<TriangleAlert aria-hidden="true" />
				<Alert.Title>This draft cannot be published yet</Alert.Title>
				<Alert.Description>
					<ul class="flex flex-col gap-1">
						{#each view.preview.blockers as blocker (`${blocker.code}:${blocker.entryId ?? ""}`)}
							<li class="break-words">{blockerMessage(blocker, view.entries)}</li>
						{/each}
					</ul>
					<a href={roadmapSettingsPath(workspace.slug)} class="mt-2 inline-block">Fix it in the roadmap settings</a>
				</Alert.Description>
			</Alert.Root>
		{/if}

		<PublicRoadmap
			roadmap={view.preview.roadmap}
			stamp="draft"
			entryHref={(id) => roadmapPreviewEntryPath(workspace.slug, id)}
		/>
	</div>
{:else}
	<RoadmapPreviewFallback {view} workspaceName={workspace.name} previewHref={roadmapPreviewPath(workspace.slug)} />
{/if}
