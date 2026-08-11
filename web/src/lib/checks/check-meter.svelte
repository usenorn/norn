<script lang="ts">
	import type { IssueCheckSummary } from "./checks";

	let { summary, class: className = "" }: { summary: IssueCheckSummary; class?: string } =
		$props();

	const settled = $derived(summary.proven + summary.waived + summary.gaps);
	const share = $derived(summary.total === 0 ? 0 : Math.round((settled / summary.total) * 100));
</script>

<span
	class="inline-flex h-1 overflow-hidden rounded-full bg-paper-3 {className}"
	role="img"
	aria-label="{settled} of {summary.total} criteria settled"
>
	<span
		class="h-full rounded-full {summary.failed > 0 ? 'bg-destructive' : 'bg-success'}"
		style="width: {share}%"
	></span>
</span>
