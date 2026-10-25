<script lang="ts">
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import { unknownLine } from "#lib/api/attempt.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { showToast } from "#lib/toast/toasts.js";
	import { memoryFailureMessage, readMemoryFailure, type Memory, type MemoryFailure } from "./memories";
	import { acceptMemory, refreshMemory } from "./memory-writes";

	let {
		memory,
		workspaceId,
		onreject,
	}: {
		memory: Memory;
		workspaceId: string;
		onreject: (memory: Memory) => void;
	} = $props();

	let accepting = $state(false);
	let failure = $state<MemoryFailure | "unknown" | null>(null);

	async function accept() {
		if (accepting) return;

		accepting = true;
		failure = null;

		const outcome = await acceptMemory(workspaceId, memory);

		accepting = false;

		if (outcome.kind === "done") {
			showToast(`“${memory.statement}” is accepted`, { detail: "Agents recall it from now on." });
			await refreshMemory(workspaceId, memory.id);

			return;
		}

		failure = outcome.kind === "unknown" ? "unknown" : readMemoryFailure(outcome.problem);
	}
</script>

<div class="flex flex-col gap-1.5">
	<div class="flex flex-wrap gap-1.5">
		<Button size="sm" disabled={accepting} onclick={accept}>
			<CircleCheck aria-hidden="true" />
			{accepting ? "Accepting" : "Accept"}
		</Button>
		<Button variant="outline" size="sm" disabled={accepting} onclick={() => onreject(memory)}>
			<CircleX aria-hidden="true" />
			Reject
		</Button>
	</div>
	{#if failure}
		<p role="alert" class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-destructive">
			<span>{failure === "unknown" ? unknownLine : memoryFailureMessage(failure)}</span>
			{#if failure !== "unknown" && failure.kind === "stale"}
				<Button variant="link" size="xs" onclick={() => refreshMemory(workspaceId, memory.id)}>Reload</Button>
			{/if}
		</p>
	{/if}
</div>
