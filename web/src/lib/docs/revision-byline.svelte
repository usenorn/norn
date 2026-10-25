<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import Pill from "#lib/components/norn/pill.svelte";
	import { revisionAuthor, type DocRevisionSummary } from "./docs";
	import type { Agent } from "#lib/agents/agents.js";

	let {
		revision,
		agents,
	}: {
		revision: DocRevisionSummary;
		agents: Agent[];
	} = $props();

	const byAgent = $derived(revision.source === "agent");
</script>

<span class="inline-flex min-w-0 items-center gap-1.5">
	<span class="min-w-0 truncate">{revisionAuthor(revision, agents)}</span>
	{#if byAgent}
		<Pill tone="active">
			<Bot aria-hidden="true" />
			<span>Agent</span>
			<span class="sr-only">made this change</span>
		</Pill>
	{/if}
</span>
