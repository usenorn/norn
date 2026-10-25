<script lang="ts">
	import type { Snippet } from "svelte";
	import X from "@lucide/svelte/icons/x";
	import { Button } from "#lib/components/ui/button/index.js";

	let {
		href,
		removeLabel,
		working = false,
		onremove,
		children,
	}: {
		href: string;
		removeLabel?: string;
		working?: boolean;
		onremove?: () => void;
		children: Snippet;
	} = $props();
</script>

<li class="flex items-center gap-1 border-b border-line-subtle">
	<a
		{href}
		class="-mx-1 flex h-row min-w-0 flex-1 items-center gap-2.25 rounded-sm px-1 motion-control hover:bg-accent"
	>
		{@render children()}
	</a>
	{#if onremove}
		<Button variant="ghost" size="sm" disabled={working} aria-label={removeLabel} onclick={onremove}>
			<X aria-hidden="true" />
		</Button>
	{/if}
</li>
