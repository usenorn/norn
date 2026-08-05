<script lang="ts">
	import { cn } from "$lib/utils.js";
	import type { LabelColor } from "./labels";

	let { color = "neutral", class: className }: { color?: LabelColor; class?: string } = $props();

	const tone = {
		neutral: "bg-label-neutral",
		cyan: "bg-label-cyan",
		blue: "bg-label-blue",
		violet: "bg-label-violet",
		orchid: "bg-label-orchid",
		magenta: "bg-label-magenta",
	};
</script>

<span class={cn("inline-block size-2 rounded-xs", tone[color], className)} aria-hidden="true"></span>
