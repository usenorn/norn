<script lang="ts">
	import { renderMarkdown } from "$lib/issues/markdown";

	let { source, class: className = "" }: { source: string; class?: string } = $props();

	const rendered = $derived(renderMarkdown(source));
</script>

{#if rendered}
	<div
		class="prose prose-sm max-w-none text-ink-900 prose-headings:font-medium prose-headings:tracking-snug prose-a:text-link prose-pre:bg-paper-2 prose-code:font-mono {className}"
	>
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html rendered}
	</div>
{/if}
