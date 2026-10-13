<script lang="ts" generics="T">
	import type { Snippet } from "svelte";

	let {
		heading,
		note,
		items,
		shown = 25,
		line,
	}: {
		heading: string;
		note: string;
		items: T[];
		shown?: number;
		line: Snippet<[T]>;
	} = $props();

	let expanded = $state(false);

	const visible = $derived(expanded ? items : items.slice(0, shown));
	const hidden = $derived(items.length - visible.length);
</script>

<section class="flex flex-col gap-2">
	<div class="flex flex-col gap-0.5">
		<h3 class="text-sm font-medium tracking-snug text-ink-900">
			{heading}
			<span class="font-mono text-xs text-muted-foreground">{items.length}</span>
		</h3>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">{note}</p>
	</div>

	{#if items.length === 0}
		<p class="text-sm text-muted-foreground">Nothing.</p>
	{:else}
		<ul class="flex flex-col gap-1 rounded-md border border-line-subtle bg-paper-0 p-3">
			{#each visible as item, index (index)}
				<li class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
					{@render line(item)}
				</li>
			{/each}
		</ul>

		{#if hidden > 0}
			<div>
				<button
					type="button"
					class="text-sm text-link underline-offset-2 hover:underline"
					onclick={() => (expanded = true)}
				>
					Show the other {hidden}
				</button>
			</div>
		{/if}
	{/if}
</section>
