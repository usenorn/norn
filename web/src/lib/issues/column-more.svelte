<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import type { IssueLayout } from "./display";
	import type { ColumnLoad } from "./paging";

	let {
		load,
		name,
		layout,
		onload,
	}: {
		load: ColumnLoad;
		name: string;
		layout: IssueLayout;
		onload: () => void;
	} = $props();
</script>

{#if load.kind !== "complete"}
	<div
		class="flex flex-col items-start gap-1.5 {layout === 'board'
			? 'px-0.5 py-1'
			: 'border-b border-line-subtle px-3 py-2'}"
	>
		{#if load.kind === "unavailable"}
			<p role="status" class="text-sm text-muted-foreground">
				We could not load any more. Nothing changed &mdash; try again.
			</p>
		{/if}
		<Button
			variant="secondary"
			size="sm"
			disabled={load.kind === "loading"}
			aria-label="Load more issues in {name}"
			onclick={onload}
		>
			{#if load.kind === "loading"}
				Loading
			{:else if load.kind === "unavailable"}
				Try again
			{:else}
				Load {load.remaining} more
			{/if}
		</Button>
	</div>
{/if}
