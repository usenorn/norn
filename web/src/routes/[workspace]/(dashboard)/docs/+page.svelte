<script lang="ts">
	import FileText from "@lucide/svelte/icons/file-text";
	import Plus from "@lucide/svelte/icons/plus";
	import { page } from "$app/state";
	import { Button } from "$lib/components/ui/button/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import DocsHeader from "$lib/docs/docs-header.svelte";
	import DocsTreePanel from "$lib/docs/docs-tree-panel.svelte";
	import { newDocPath } from "$lib/docs/docs";
	import { docsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const tree = $derived(
		(import.meta.env.DEV ? docsPreviewStates[page.url.searchParams.get("state") ?? ""]?.tree : undefined) ??
			data.tree
	);
</script>

<svelte:head>
	<title>Docs · {data.workspace.name} · Norn</title>
</svelte:head>

<DocsHeader workspace={data.workspace.slug}>
	{#snippet actions()}
		<Button size="sm" href={newDocPath(data.workspace.slug)} class="md:hidden">
			<Plus aria-hidden="true" />
			New doc
		</Button>
	{/snippet}
</DocsHeader>

<div class="flex-1 overflow-y-auto">
	<div class="px-2 py-3 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))] md:hidden">
		<DocsTreePanel workspace={data.workspace.slug} {tree} size="touch" />
	</div>
	<div class="mx-auto hidden w-full max-w-180 px-4 py-10 md:block">
		{#if tree.kind === "ready"}
			<Empty.Root>
				<Empty.Media variant="icon"><FileText aria-hidden="true" /></Empty.Media>
				<Empty.Header>
					<Empty.Title>Pick a doc to read</Empty.Title>
					<Empty.Description>
						Docs shared with the whole workspace, with a project, or kept for yourself are listed on
						the left.
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button variant="secondary" href={newDocPath(data.workspace.slug)}>
						<Plus aria-hidden="true" />
						New doc
					</Button>
				</Empty.Content>
			</Empty.Root>
		{:else if tree.kind === "empty"}
			<DocsTreePanel workspace={data.workspace.slug} {tree} />
		{/if}
	</div>
</div>
