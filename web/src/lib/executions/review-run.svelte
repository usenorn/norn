<script lang="ts">
	import RunRow from "./run-row.svelte";
	import { waitingOnLine, type ReviewRun } from "./reviews";

	let { item, href, timezone }: { item: ReviewRun; href: string; timezone: string } = $props();
</script>

<div class="flex min-w-0 flex-col border-b border-line-subtle last:border-b-0">
	<RunRow execution={item.run.execution} change={item.run.change} {href} {timezone} class="border-b-0" />
	{#if !item.decision.canDecide}
		<p class="px-1 pb-2 text-xs text-muted-foreground text-pretty">{waitingOnLine(item.decision)}</p>
	{/if}
</div>
