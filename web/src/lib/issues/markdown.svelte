<script lang="ts">
	import { markdownProse, renderMarkdown } from "$lib/issues/markdown";

	let { source, class: className = "" }: { source: string; class?: string } = $props();

	const rendered = $derived(renderMarkdown(source));
</script>

{#if rendered}
	<div
		class="{markdownProse} {className}"
	>
		<!-- eslint-disable-next-line svelte/no-at-html-tags -->
		{@html rendered}
	</div>
{/if}
