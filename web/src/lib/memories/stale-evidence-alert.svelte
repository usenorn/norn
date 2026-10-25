<script lang="ts">
	import FileExclamationPoint from "@lucide/svelte/icons/file-exclamation-point";
	import { unknownLine } from "#lib/api/attempt.js";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { showToast } from "#lib/toast/toasts.js";
	import {
		memoryEditPath,
		memoryFailureMessage,
		memorySupersedePath,
		readMemoryFailure,
		type EvidenceHealth,
		type Memory,
		type MemoryFailure,
	} from "./memories";
	import { confirmMemoryEvidence, refreshMemory } from "./memory-writes";

	let {
		memory,
		health,
		workspace,
		workspaceId,
		supersedable,
	}: {
		memory: Memory;
		health: Exclude<EvidenceHealth, { kind: "current" }>;
		workspace: string;
		workspaceId: string;
		supersedable: boolean;
	} = $props();

	let confirming = $state(false);
	let failure = $state<MemoryFailure | "unknown" | null>(null);

	async function confirm() {
		if (confirming) return;

		confirming = true;
		failure = null;

		const outcome = await confirmMemoryEvidence(workspaceId, memory);

		confirming = false;

		if (outcome.kind === "done") {
			showToast("Its sources are confirmed", { detail: "The memory now rests on their current versions." });
			await refreshMemory(workspaceId, memory.id);

			return;
		}

		failure = outcome.kind === "unknown" ? "unknown" : readMemoryFailure(outcome.problem);
	}

	async function reload() {
		failure = null;
		await refreshMemory(workspaceId, memory.id);
	}
</script>

<Alert.Root variant="warning">
	<FileExclamationPoint aria-hidden="true" />
	{#if health.kind === "changed"}
		<Alert.Title>A source behind this memory changed after it was recorded</Alert.Title>
	{:else}
		<Alert.Title>A source behind this memory is no longer available</Alert.Title>
	{/if}
	<Alert.Description>
		{#if memory.viewer.canEdit && health.kind === "changed"}
			It still guides agents as it reads. Confirm it if its sources still hold it up, correct it, or supersede
			it.
		{:else if memory.viewer.canEdit}
			It still guides agents as it reads. Correct it to rest on what you can still cite, or supersede it.
		{:else}
			It still guides agents as it reads. Its editors have been told so they can confirm, correct, or
			supersede it.
		{/if}
		{#if failure}
			<span role="alert" class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-destructive">
				{failure === "unknown" ? unknownLine : memoryFailureMessage(failure)}
				{#if failure !== "unknown" && failure.kind === "stale"}
					<Button variant="link" size="xs" onclick={reload}>Reload</Button>
				{/if}
			</span>
		{/if}
	</Alert.Description>
	{#if memory.viewer.canEdit}
		<Alert.Action placement="below">
			{#if health.kind === "changed"}
				<Button variant="secondary" size="sm" disabled={confirming} onclick={confirm}>
					{confirming ? "Confirming" : "Confirm sources"}
				</Button>
			{/if}
			<Button variant="secondary" size="sm" href={memoryEditPath(workspace, memory.id)} aria-disabled={confirming}>
				Correct
			</Button>
			{#if supersedable}
				<Button
					variant="secondary"
					size="sm"
					href={memorySupersedePath(workspace, memory.id)}
					aria-disabled={confirming}
				>
					Supersede
				</Button>
			{/if}
		</Alert.Action>
	{/if}
</Alert.Root>
