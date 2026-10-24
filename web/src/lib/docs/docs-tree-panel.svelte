<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import FileText from "@lucide/svelte/icons/file-text";
	import Plus from "@lucide/svelte/icons/plus";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import DocTree from "./doc-tree.svelte";
	import { newDocPath, type DocTree as Tree } from "./docs";

	let {
		workspace,
		tree,
		currentId,
		size = "row",
		placement = "page",
	}: {
		workspace: string;
		tree: Tree;
		currentId?: string;
		size?: "row" | "touch";
		placement?: "page" | "sidebar";
	} = $props();
</script>

{#if tree.kind === "loading"}
	<div class="flex flex-col gap-1.5" aria-busy="true" aria-label="Loading docs">
		{#each [0, 1, 2, 3] as row (row)}
			<div class="h-6.75 animate-breathe rounded-xs bg-paper-2"></div>
		{/each}
	</div>
{:else if tree.kind === "unavailable"}
	<Alert.Root variant="destructive">
		<CircleX aria-hidden="true" />
		<Alert.Title>We could not load the docs</Alert.Title>
		<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
	</Alert.Root>
{:else if tree.kind === "empty" && placement === "sidebar"}
	<p class="px-2 text-xs text-muted-foreground">No docs yet.</p>
{:else if tree.kind === "empty"}
	<Empty.Root>
		<Empty.Media variant="icon"><FileText aria-hidden="true" /></Empty.Media>
		<Empty.Header>
			<Empty.Title>No docs yet</Empty.Title>
			<Empty.Description>
				Write down what the team keeps having to explain — runbooks, decisions, how things work.
			</Empty.Description>
		</Empty.Header>
		<Empty.Content>
			<Button href={newDocPath(workspace)}>
				<Plus aria-hidden="true" />
				New doc
			</Button>
		</Empty.Content>
	</Empty.Root>
{:else}
	<DocTree {workspace} sections={tree.sections} docs={tree.docs} {currentId} {size} scope={placement} />
	{#if tree.partial}
		<p class="mt-3 px-2 text-xs text-muted-foreground">
			Only the most recently changed docs are listed here. Search to find the rest.
		</p>
	{/if}
{/if}
