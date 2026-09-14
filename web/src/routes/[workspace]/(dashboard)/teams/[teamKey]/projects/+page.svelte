<script lang="ts">
	import { page } from "$app/state";
	import ProjectsScreen from "$lib/projects/projects-screen.svelte";
	import { teamProjectsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? teamProjectsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const listing = $derived(preview?.listing ?? data.listing);
</script>

<ProjectsScreen workspace={data.workspace} {listing} team={data.team} />
