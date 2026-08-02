<script lang="ts">
	import { cn } from "$lib/utils.js";

	let { keys, class: className }: { keys: string; class?: string } = $props();
</script>

<kbd
	class={cn(
		"keycap inline-flex h-4.5 min-w-4.5 items-center justify-center gap-1 rounded-xs border border-line-strong bg-paper-2 px-1 font-mono text-2xs font-medium text-ink-600",
		className
	)}
>
	{keys}
</kbd>
