<script lang="ts">
	import { totalIssues, type IssueProgress } from "$lib/issues/board";
	import { progressLabel, scopeSegments } from "./projects";
	import { cn } from "$lib/utils.js";

	let { progress, class: className }: { progress: IssueProgress; class?: string } = $props();

	const total = $derived(totalIssues(progress));
	const segments = $derived(scopeSegments(progress));
</script>

<span class={cn("flex min-w-0 items-center gap-2.5", className)}>
	<span
		class="flex h-1.5 min-w-0 flex-1 overflow-hidden rounded-xs bg-paper-3"
		role="progressbar"
		aria-valuenow={progress.complete}
		aria-valuemin={0}
		aria-valuemax={total}
		aria-label="{progress.complete} of {total} done"
	>
		{#each segments as segment (segment.label)}
			<span
				class="block h-1.5 bg-(--tone)"
				style="width: {(100 * segment.count) / total}%; --tone: var(--color-{segment.token})"
				title="{segment.count} {segment.label.toLowerCase()}"
			></span>
		{/each}
	</span>
	<span class="shrink-0 font-mono text-xs whitespace-nowrap text-muted-foreground tabular-nums">
		{progressLabel(progress)}
	</span>
</span>
