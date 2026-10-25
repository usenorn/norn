<script lang="ts">
	import ReviewList from "#lib/components/norn/review-list.svelte";
	import { outcomeLabel, resourceLabel, type ImportPreviewLine } from "./imports";

	let {
		heading,
		note,
		lines,
		shown = 25,
	}: {
		heading: string;
		note: string;
		lines: ImportPreviewLine[];
		shown?: number;
	} = $props();
</script>

<ReviewList {heading} {note} items={lines} {shown}>
	{#snippet line(item: ImportPreviewLine)}
		<span class="font-mono text-xs text-muted-foreground">{resourceLabel(item.resource)}</span>
		<span class="text-sm text-ink-900">{item.subject ?? item.externalId ?? "—"}</span>
		<span class="text-xs text-muted-foreground">{outcomeLabel(item.outcome)}</span>
		{#if item.detail}
			<span class="text-xs text-muted-foreground text-pretty">{item.detail}</span>
		{/if}
	{/snippet}
</ReviewList>
