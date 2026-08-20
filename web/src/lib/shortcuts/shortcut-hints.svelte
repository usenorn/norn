<script lang="ts">
	import Kbd from "$lib/components/norn/kbd.svelte";
	import { cn } from "$lib/utils.js";
	import { useShortcuts } from "./registry.svelte";
	import { isApplePlatform, keycap, shortcutOf, type ShortcutId } from "./shortcuts";

	let {
		ids,
		class: className,
	}: { ids: readonly ShortcutId[]; class?: string } = $props();

	const registry = useShortcuts();
	const apple = isApplePlatform();

	const shown = $derived(
		ids.filter((id) => registry.bound(id)).map((id) => shortcutOf(id))
	);
</script>

{#if shown.length > 0}
	<div class={cn("flex flex-wrap items-center gap-x-3 gap-y-1.5", className)}>
		{#each shown as shortcut (shortcut.id)}
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys={keycap(shortcut, apple)} />{shortcut.label}
			</span>
		{/each}
	</div>
{/if}
