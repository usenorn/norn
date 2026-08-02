<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils.js";

	let {
		label,
		action,
		class: className,
	}: { label: string; action?: Snippet; class?: string } = $props();
</script>

<div class={cn("mt-4 mb-1 flex items-center gap-2 px-2", className)}>
	<span class="font-mono text-2xs font-medium tracking-caps text-ink-600 uppercase">{label}</span>
	<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
	{@render action?.()}
</div>
