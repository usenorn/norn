<script lang="ts">
	import ChevronLeft from "@lucide/svelte/icons/chevron-left";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import { cn } from "$lib/utils.js";

	let {
		side,
		label,
		onrestore,
		class: className,
	}: {
		side: "leading" | "trailing";
		label: string;
		onrestore: () => void;
		class?: string;
	} = $props();

	const Glyph = $derived(side === "leading" ? ChevronRight : ChevronLeft);
</script>

<button
	type="button"
	onclick={onrestore}
	aria-label={label}
	class={cn(
		"absolute top-1/2 z-10 flex h-11 w-4 -translate-y-1/2 items-center justify-center border border-line-default bg-card text-ink-600 motion-control hover:bg-accent hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
		side === "leading" ? "left-0 rounded-r-sm border-l-0" : "right-0 rounded-l-sm border-r-0",
		className
	)}
>
	<Glyph class="size-3" aria-hidden="true" />
</button>
