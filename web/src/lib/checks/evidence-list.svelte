<script lang="ts">
	import EvidenceRecord from "./evidence-record.svelte";
	import type { EvidencePanel } from "./checks";

	let { panel, timezone }: { panel: EvidencePanel; timezone: string } = $props();
</script>

{#if panel.kind === "loading"}
	<p class="text-xs text-muted-foreground">Reading what has been filed…</p>
{:else if panel.kind === "empty"}
	<p class="text-xs leading-normal text-muted-foreground text-pretty">
		Nothing has been filed against this yet.
	</p>
{:else if panel.kind === "unavailable"}
	<p class="text-xs leading-normal text-muted-foreground text-pretty">
		We could not read the evidence. Try again in a moment.
	</p>
{:else}
	<ol class="flex min-w-0 flex-col gap-2.5">
		{#each panel.evidence as record (record.id)}
			<EvidenceRecord {record} {timezone} />
		{/each}
	</ol>
{/if}
