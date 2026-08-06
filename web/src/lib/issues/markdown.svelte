<script lang="ts">
	import { renderMarkdown } from "$lib/issues/markdown";

	let { source, class: className = "" }: { source: string; class?: string } = $props();

	const rendered = $derived(renderMarkdown(source));
</script>

{#if rendered}
	<div
		class="prose prose-sm max-w-none text-ink-900 [--tw-prose-bold:var(--ink-900)] [--tw-prose-bullets:var(--line-strong)] [--tw-prose-code:var(--ink-900)] [--tw-prose-counters:var(--text-muted)] [--tw-prose-headings:var(--ink-900)] [--tw-prose-quote-borders:var(--line-strong)] [--tw-prose-quotes:var(--ink-600)] prose-headings:font-medium prose-headings:tracking-snug prose-a:text-link prose-pre:bg-paper-2 prose-code:font-mono prose-img:my-3 prose-img:max-h-[32rem] prose-img:w-auto prose-img:rounded-md prose-img:border prose-img:border-line-subtle prose-img:bg-paper-2 prose-img:object-contain {className}"
	>
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html rendered}
	</div>
{/if}
