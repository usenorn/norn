<script lang="ts">
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import { cn } from "$lib/utils.js";

	let {
		name,
		logoUrl,
		class: className,
	}: { name: string; logoUrl?: string; class?: string } = $props();

	const initial = $derived(name.trim().charAt(0).toUpperCase() || "?");
</script>

<Avatar.Root
	variant="mark"
	shape="square"
	class={cn("size-5.5 text-md", className)}
	aria-hidden="true"
>
	{#if logoUrl}
		<Avatar.Image src={logoUrl} alt="" />
	{/if}
	<Avatar.Fallback>{initial}</Avatar.Fallback>
</Avatar.Root>
