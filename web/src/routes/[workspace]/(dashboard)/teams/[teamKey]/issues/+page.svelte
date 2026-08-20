<script lang="ts">
	import { page } from "$app/state";
	import IssueListing from "$lib/issues/issue-listing.svelte";
	import { teamIssuesPath } from "$lib/issues/listing";
	import { teamIssuesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? teamIssuesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	const teamKey = $derived(page.params.teamKey ?? "");
</script>

<svelte:head>
	<title>{data.team?.name ?? "Team"} issues · {data.workspace.name} · Norn</title>
</svelte:head>

<IssueListing {data} {preview} basePath={teamIssuesPath(data.workspace.slug, teamKey)} />
