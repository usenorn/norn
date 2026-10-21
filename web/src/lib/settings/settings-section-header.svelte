<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		id,
		title,
		count,
		badge,
		children,
	}: {
		id: string;
		title: string;
		count?: number;
		badge?: Snippet;
		children: Snippet;
	} = $props();
</script>

<div class="flex min-w-0 flex-col gap-0.5">
	<div class="flex flex-wrap items-center gap-2">
		<h2 {id} class="text-md font-semibold tracking-snug text-ink-900">
			{title}
			{#if count !== undefined}
				<span class="tabular-nums text-xs text-muted-foreground">{count}</span>
			{/if}
		</h2>
		{@render badge?.()}
	</div>
	<p class="text-sm leading-normal text-muted-foreground text-pretty">{@render children()}</p>
</div>
