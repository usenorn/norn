<script lang="ts">
	import type { Snippet } from "svelte";
	import RoamIndicator from "./roam/roam-indicator.svelte";
	import ShortcutHints from "./shortcut-hints.svelte";
	import type { ShortcutId } from "./shortcuts";

	let { ids, lead }: { ids: readonly ShortcutId[]; lead?: Snippet } = $props();
</script>

<div
	class="hidden h-8.5 flex-none items-center gap-4 border-t border-line-subtle bg-card px-3.5 md:flex"
>
	{#if lead}
		{@render lead()}
	{/if}
	<span class="flex-1"></span>
	<ShortcutHints {ids} />
	<RoamIndicator />
</div>
