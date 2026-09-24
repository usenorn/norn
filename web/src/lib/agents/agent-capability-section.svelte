<script lang="ts">
	import type { Snippet } from "svelte";

	let {
		title,
		description,
		emptyLine,
		empty,
		actions,
		children,
	}: {
		title: string;
		description: string;
		emptyLine: string;
		empty: boolean;
		actions?: Snippet;
		children?: Snippet;
	} = $props();
</script>

<section class="grid gap-3 border-b border-line-subtle p-4 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_auto]">
	<div class="min-w-0">
		<h3 class="text-sm font-medium tracking-snug text-ink-900">{title}</h3>
		<p class="mt-0.5 text-xs leading-normal text-muted-foreground text-pretty">{description}</p>
	</div>
	{#if actions}
		<div class="flex flex-wrap gap-2 sm:row-span-2 sm:justify-end">
			{@render actions()}
		</div>
	{/if}
	<div class="min-w-0 sm:col-span-2">
		{#if !empty && children}
			{@render children()}
		{:else}
			<p class="text-xs text-muted-foreground">{emptyLine}</p>
		{/if}
	</div>
</section>
