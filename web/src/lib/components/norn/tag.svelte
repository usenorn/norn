<script lang="ts">
	import LabelDot from "$lib/labels/label-dot.svelte";
	import { cn } from "$lib/utils.js";
	import type { LabelColor } from "$lib/labels/labels";

	let {
		name,
		color = "neutral",
		class: className,
	}: { name: string; color?: LabelColor; class?: string } = $props();
</script>

<span
	title={name}
	class={cn(
		"inline-flex max-w-36 min-w-0 shrink items-center gap-1.5 text-xs font-medium text-ink-700",
		className
	)}
>
	<LabelDot {color} class="size-1.75 flex-none" />
	<span class="truncate">{name}</span>
</span>
