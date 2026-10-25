<script lang="ts">
	import Check from "@lucide/svelte/icons/check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { page } from "$app/state";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import DocEditor from "#lib/docs/doc-editor.svelte";
	import DocsHeader from "#lib/docs/docs-header.svelte";
	import { ancestry, docPath, docsPath } from "#lib/docs/docs.js";
	import { docEditPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV ? docEditPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);
	const editing = $derived(preview?.editing ?? data.editing);
	const doc = $derived(editing.kind === "ready" ? editing.doc : null);
	const known = $derived(data.tree.kind === "ready" ? data.tree.docs : []);
	const trail = $derived(doc ? [...ancestry(known, doc.id), doc] : []);
</script>

<svelte:head>
	<title>Editing {doc ? doc.title : "doc"} · {data.workspace.name} · Norn</title>
</svelte:head>

<DocsHeader workspace={slug} {trail} title="Editing">
	{#snippet actions()}
		{#if doc}
			<Button size="sm" href={docPath(slug, doc.id)}>
				<Check aria-hidden="true" />
				Done
			</Button>
		{/if}
	{/snippet}
</DocsHeader>

<div class="flex-1 overflow-y-auto">
	<div
		class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-4 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
	>
		{#if editing.kind === "loading"}
			<div class="h-8 w-2/3 animate-breathe rounded-md bg-paper-2" aria-busy="true"></div>
			<div class="h-80 animate-breathe rounded-lg bg-paper-2" aria-hidden="true"></div>
		{:else if editing.kind === "not_found"}
			<div class="flex flex-col gap-2">
				<h2 class="text-md font-medium tracking-snug text-ink-900">No doc here</h2>
				<p class="text-sm leading-normal text-pretty text-muted-foreground">
					There is no doc at this address in {data.workspace.name}, or it is not shared with you.
				</p>
				<div>
					<Button variant="secondary" size="sm" href={docsPath(slug)}>All docs</Button>
				</div>
			</div>
		{:else if editing.kind === "unavailable" || !doc}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not open this doc for editing</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		{:else}
			{#key doc.id}
				<DocEditor
					{doc}
					workspaceId={data.workspace.id}
					workspace={slug}
					shownSave={preview?.save}
					shownConflict={preview?.conflict}
				/>
			{/key}
		{/if}
	</div>
</div>
