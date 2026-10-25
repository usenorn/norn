<script lang="ts">
	import { SvelteSet } from "svelte/reactivity";
	import FileText from "@lucide/svelte/icons/file-text";
	import SidebarBranch from "#lib/components/norn/sidebar-branch.svelte";
	import SidebarItem from "#lib/components/norn/sidebar-item.svelte";
	import SidebarSection from "#lib/components/norn/sidebar-section.svelte";
	import { ancestry, docPath, type DocNode, type DocSection, type DocSummary } from "./docs";

	let {
		workspace,
		sections,
		docs,
		currentId,
		size = "row",
		scope,
	}: {
		workspace: string;
		sections: DocSection[];
		docs: DocSummary[];
		currentId?: string;
		size?: "row" | "touch";
		scope: string;
	} = $props();

	const toggled = new SvelteSet<string>();
	const opened = $derived(new Set(currentId ? ancestry(docs, currentId).map((doc) => doc.id) : []));

	function expanded(docId: string): boolean {
		return opened.has(docId) !== toggled.has(docId);
	}

	function toggle(docId: string) {
		if (toggled.has(docId)) toggled.delete(docId);
		else toggled.add(docId);
	}
</script>

{#snippet branch(nodes: DocNode[])}
	<ul class="flex flex-col">
		{#each nodes as node (node.doc.id)}
			<li>
				{#if node.children.length > 0}
					<SidebarBranch
						id="doc-tree-{scope}-{node.doc.id}"
						href={docPath(workspace, node.doc.id)}
						label={node.doc.title}
						icon={FileText}
						active={node.doc.id === currentId}
						expanded={expanded(node.doc.id)}
						ontoggle={() => toggle(node.doc.id)}
						{size}
					>
						<div class="pl-3">
							{@render branch(node.children)}
						</div>
					</SidebarBranch>
				{:else}
					<SidebarItem
						href={docPath(workspace, node.doc.id)}
						label={node.doc.title}
						icon={FileText}
						active={node.doc.id === currentId}
						{size}
					/>
				{/if}
			</li>
		{/each}
	</ul>
{/snippet}

<nav aria-label="Docs" class="flex min-w-0 flex-col">
	{#each sections as section (section.key)}
		<SidebarSection label={section.title} class="first:mt-0" />
		{@render branch(section.nodes)}
	{/each}
</nav>
