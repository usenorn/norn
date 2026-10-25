<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Lock from "@lucide/svelte/icons/lock";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { memoriesPath, memoryPath, memoryStatusLabels, type MemoryDraft } from "./memories";

	let {
		draft,
		workspace,
		workspaceName,
	}: {
		draft: Exclude<MemoryDraft, { kind: "ready" }>;
		workspace: string;
		workspaceName: string;
	} = $props();
</script>

{#if draft.kind === "loading"}
	<div class="h-8 w-2/3 animate-breathe rounded-md bg-paper-2" aria-busy="true"></div>
	<div class="h-80 animate-breathe rounded-lg bg-paper-2" aria-hidden="true"></div>
{:else if draft.kind === "not_found"}
	<div class="flex flex-col gap-2">
		<h2 class="text-md font-medium tracking-snug text-ink-900">No memory here</h2>
		<p class="text-sm leading-normal text-pretty text-muted-foreground">
			There is no memory at this address in {workspaceName}, or it is not shared with you.
		</p>
		<div>
			<Button variant="secondary" size="sm" href={memoriesPath(workspace)}>All memories</Button>
		</div>
	</div>
{:else if draft.kind === "unavailable"}
	<Alert.Root variant="destructive">
		<CircleX aria-hidden="true" />
		<Alert.Title>We could not load this memory</Alert.Title>
		<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
	</Alert.Root>
{:else}
	<Alert.Root>
		<Lock aria-hidden="true" />
		{#if draft.kind === "read_only"}
			<Alert.Title>You can read this memory</Alert.Title>
			<Alert.Description>Only its author, its contributors, and its maintainers can change it.</Alert.Description>
		{:else}
			<Alert.Title>This memory takes no more changes</Alert.Title>
			<Alert.Description>
				It is {memoryStatusLabels[draft.memory.status].toLowerCase()}. Record a new memory if something else
				holds now.
			</Alert.Description>
		{/if}
		<Alert.Action>
			<Button variant="secondary" size="sm" href={memoryPath(workspace, draft.memory.id)}>Back to the memory</Button>
		</Alert.Action>
	</Alert.Root>
{/if}
