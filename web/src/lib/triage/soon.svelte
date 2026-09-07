<script lang="ts">
	import { cn } from "$lib/utils.js";

	let { class: className }: { class?: string } = $props();
</script>

<span
	class={cn(
		"inline-flex h-4 flex-none items-center rounded-xs border border-line-subtle px-1 font-mono text-2xs font-medium tracking-eyebrow text-muted-foreground uppercase",
		className
	)}
>
	Soon
</span>
