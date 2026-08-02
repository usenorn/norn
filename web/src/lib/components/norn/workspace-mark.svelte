<script lang="ts">
	import { cn } from "$lib/utils.js";

	let { name, class: className }: { name: string; class?: string } = $props();

	const initial = $derived(name.trim().charAt(0).toUpperCase() || "?");
</script>

<span
	class={cn(
		"inline-flex size-5.5 shrink-0 items-center justify-center rounded-sm bg-primary text-md font-medium text-primary-foreground",
		className
	)}
	aria-hidden="true"
>
	{initial}
</span>
