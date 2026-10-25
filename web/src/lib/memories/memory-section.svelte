<script lang="ts">
	import type { Snippet } from "svelte";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";

	let { title, children }: { title: string; children: Snippet } = $props();
</script>

<section class="flex min-w-0 flex-col gap-1.5">
	<h3><Eyebrow rule class="text-ink-600">{title}</Eyebrow></h3>
	{@render children()}
</section>
