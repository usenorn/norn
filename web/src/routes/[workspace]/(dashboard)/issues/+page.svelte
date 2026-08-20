<script lang="ts">
	import { page } from "$app/state";
	import IssueListing from "$lib/issues/issue-listing.svelte";
	import { issuesPath } from "$lib/issues/listing";
	import { issuesPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? issuesPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
</script>

<svelte:head><title>Issues · Norn</title></svelte:head>

<IssueListing {data} {preview} basePath={issuesPath(data.workspace.slug)} />
