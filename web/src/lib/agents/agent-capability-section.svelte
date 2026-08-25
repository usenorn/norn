<script lang="ts">
	import type { Snippet } from "svelte";
	import { Button } from "$lib/components/ui/button/index.js";

	let {
		title,
		description,
		emptyLine,
		empty,
		action,
		disabled = false,
		onadd,
		children,
	}: {
		title: string;
		description: string;
		emptyLine: string;
		empty: boolean;
		action: string;
		disabled?: boolean;
		onadd?: () => void;
		children?: Snippet;
	} = $props();
</script>

<section class="grid gap-3 border-b border-line-subtle p-4 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_auto]">
	<div class="min-w-0">
		<h3 class="text-sm font-medium tracking-snug text-ink-900">{title}</h3>
		<p class="mt-0.5 text-xs leading-normal text-muted-foreground text-pretty">{description}</p>
	</div>
	{#if onadd}
		<div class="sm:row-span-2">
			<Button variant="secondary" size="sm" {disabled} onclick={onadd}>{action}</Button>
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
