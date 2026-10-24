<script lang="ts">
	import Plus from "@lucide/svelte/icons/plus";
	import { page } from "$app/state";
	import { Button } from "$lib/components/ui/button/index.js";
	import DocsTreePanel from "$lib/docs/docs-tree-panel.svelte";
	import { newDocPath } from "$lib/docs/docs";
	import { docsPreviewStates } from "./preview";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const tree = $derived(
		(import.meta.env.DEV && page.route.id?.endsWith("/docs")
			? docsPreviewStates[page.url.searchParams.get("state") ?? ""]?.tree
			: undefined) ?? data.tree
	);
</script>

<div class="flex min-h-0 flex-1">
	<aside
		aria-label="All docs"
		class="hidden w-64 flex-none flex-col border-r border-line-default md:flex"
	>
		<div class="flex h-11 flex-none items-center gap-2 border-b border-line-default pr-2 pl-4">
			<span class="caption flex-1 text-ink-600">All docs</span>
			<Button variant="ghost" size="sm" href={newDocPath(data.workspace.slug)}>
				<Plus aria-hidden="true" />
				New doc
			</Button>
		</div>
		<div class="min-h-0 flex-1 overflow-y-auto px-2 py-3">
			<DocsTreePanel workspace={data.workspace.slug} {tree} currentId={page.params.docId} placement="sidebar" />
		</div>
	</aside>
	<div class="flex min-h-0 min-w-0 flex-1 flex-col">
		{@render children()}
	</div>
</div>
