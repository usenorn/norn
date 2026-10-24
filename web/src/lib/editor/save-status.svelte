<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import { saveLine, type SaveState } from "./autosave.svelte";

	let { standing, onretry }: { standing: SaveState; onretry: () => void } = $props();

	const line = $derived(saveLine(standing));
	const quiet = $derived(standing.kind === "saved" || standing.kind === "waiting" || standing.kind === "saving");
</script>

<span class="text-xs {quiet ? 'text-muted-foreground' : 'text-destructive'}" aria-live="polite">{line}</span>
{#if standing.kind === "failed" || standing.kind === "unknown"}
	<Button type="button" variant="secondary" size="sm" onclick={onretry}>Try again</Button>
{/if}
