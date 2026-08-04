<script lang="ts">
	import { cn } from "$lib/utils.js";

	let { key, class: className }: { key: string; class?: string } = $props();
</script>

<span
	class={cn(
		"shrink-0 font-mono text-xs font-medium tracking-caps text-ink-600 uppercase",
		className
	)}
>
	{key}
</span>
