<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { memoryFailureMessage, memoryPath, type MemoryFailure } from "./memories";

	let {
		failure,
		title,
		retry,
		workspace,
		memoryId,
	}: {
		failure: MemoryFailure;
		title: string;
		retry: string;
		workspace: string;
		memoryId: string;
	} = $props();
</script>

{#if failure.kind === "stale"}
	<Alert.Root variant="warning">
		<TriangleAlert aria-hidden="true" />
		<Alert.Title>Somebody changed this memory while you were writing</Alert.Title>
		<Alert.Description>
			Nothing you wrote was saved over it. Read what it says now, then make your change again.
		</Alert.Description>
		<Alert.Action>
			<Button variant="secondary" size="sm" href={retry} data-sveltekit-reload>Start from the latest</Button>
			<Button variant="ghost" size="sm" href={memoryPath(workspace, memoryId)}>Back to the memory</Button>
		</Alert.Action>
	</Alert.Root>
{:else}
	<Alert.Root variant="destructive">
		<CircleX aria-hidden="true" />
		<Alert.Title>{title}</Alert.Title>
		<Alert.Description>{memoryFailureMessage(failure)}</Alert.Description>
	</Alert.Root>
{/if}
