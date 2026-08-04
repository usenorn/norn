<script lang="ts">
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import { totalIssues, type IssueProgress } from "$lib/issues/board";
	import type { Issue } from "$lib/issues/issues";

	let {
		children,
		progress,
		at,
	}: {
		children: Issue[];
		progress: IssueProgress;
		at: (path: string) => string;
	} = $props();

	const counted = $derived(totalIssues(progress));
</script>

<section class="flex flex-col gap-2">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 class="text-sm font-medium text-ink-900">Children</h2>
		{#if counted > 0}
			<ProgressBar {progress} />
		{/if}
	</div>

	{#if children.length === 0}
		<p class="text-sm text-muted-foreground">Nothing is filed under this issue yet.</p>
	{:else}
		<ul class="flex flex-col rounded-lg border border-line-default">
			{#each children as child (child.id)}
				<li class="border-b border-line-subtle last:border-b-0">
					<a
						href={at(`/issues/${child.reference}`)}
						class="flex items-center gap-2 px-3 py-2 transition-colors duration-70 ease-out hover:bg-accent"
					>
						<StatusIcon category={child.state.category} name={child.state.name} />
						<span class="w-15 flex-none font-mono text-xs text-muted-foreground">
							{child.reference}
						</span>
						<span class="min-w-0 flex-1 truncate text-md tracking-snug text-ink-900">
							{child.title}
						</span>
						<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">
							{child.state.name}
						</span>
					</a>
				</li>
			{/each}
		</ul>

		{#if children.length > counted}
			<p class="text-sm text-muted-foreground">
				{children.length - counted} of these are archived or deleted, so they are not counted in
				the figure above.
			</p>
		{/if}
	{/if}
</section>
