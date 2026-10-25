<script lang="ts">
	import { cn } from "#lib/utils.js";

	let { class: className }: { class?: string } = $props();
</script>

<span
	class={cn(
		"inline-flex h-4 flex-none items-center rounded-xs border border-line-subtle px-1 text-xs font-medium text-muted-foreground",
		className
	)}
>
	Soon
</span>
