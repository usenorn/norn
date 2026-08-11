<script lang="ts">
	import CheckMeter from "./check-meter.svelte";
	import { summaryLine, type IssueCheckSummary } from "./checks";

	let { summary }: { summary: IssueCheckSummary } = $props();
</script>

<span class="inline-flex items-center gap-1.75">
	<span class="font-mono text-xs text-muted-foreground">{summaryLine(summary)}</span>
	{#if summary.total > 0}
		<CheckMeter {summary} class="w-14" />
	{/if}
</span>
