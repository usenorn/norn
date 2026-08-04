<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { totalIssues, type IssueProgress } from "$lib/issues/board";

	let {
		progress,
		label = true,
		class: className,
	}: { progress: IssueProgress; label?: boolean; class?: string } = $props();

	const total = $derived(totalIssues(progress));
	const done = $derived(total === 0 ? 0 : Math.round((progress.complete / total) * 100));
</script>

{#if total > 0}
	<span class={cn("inline-flex shrink-0 items-center gap-2 whitespace-nowrap", className)}>
		<span
			class="inline-block h-[3px] w-16 overflow-hidden rounded-xs bg-paper-3"
			role="progressbar"
			aria-valuenow={progress.complete}
			aria-valuemin={0}
			aria-valuemax={total}
			aria-label="{progress.complete} of {total} done"
		>
			<span class="block h-[3px] rounded-xs bg-primary" style="width: {done}%"></span>
		</span>
		{#if label}
			<span class="font-mono text-xs text-muted-foreground tabular-nums">
				{progress.complete}/{total} done
			</span>
		{/if}
	</span>
{/if}
