<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils.js";
	import Eyebrow from "./eyebrow.svelte";

	let {
		label,
		title,
		id,
		class: className,
		children,
	}: { label: string; title: string; id?: string; class?: string; children: Snippet } = $props();
</script>

<section
	{id}
	class={cn(
		"flex min-w-0 scroll-mt-4 flex-col gap-2 rounded-sm border border-amber-700/40 bg-amber-500/5 p-3 dark:border-amber-400/30",
		className
	)}
	aria-label={label}
>
	<Eyebrow tone="attention">{title}</Eyebrow>
	{@render children()}
</section>
