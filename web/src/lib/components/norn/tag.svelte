<script lang="ts">
	import { cn } from "$lib/utils.js";
	import type { LabelColor } from "$lib/labels/labels";

	let {
		name,
		color = "neutral",
		class: className,
	}: { name: string; color?: LabelColor; class?: string } = $props();

	const tone = {
		neutral: "border-b-label-neutral",
		cyan: "border-b-label-cyan",
		blue: "border-b-label-blue",
		violet: "border-b-label-violet",
		orchid: "border-b-label-orchid",
		magenta: "border-b-label-magenta",
	};
</script>

<span
	class={cn(
		"inline-flex h-4.25 max-w-26 shrink-0 items-center truncate border-b-2 px-px font-mono text-2xs font-medium tracking-wide text-muted-foreground uppercase",
		tone[color],
		className
	)}
>
	{name}
</span>
