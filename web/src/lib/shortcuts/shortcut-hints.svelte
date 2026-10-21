<script lang="ts">
	import Kbd from "$lib/components/norn/kbd.svelte";
	import { cn } from "$lib/utils.js";
	import { useShortcuts } from "./registry.svelte";
	import {
		displayKeys,
		isApplePlatform,
		keycap,
		shortcutOf,
		type Shortcut,
		type ShortcutId,
	} from "./shortcuts";

	let {
		ids,
		class: className,
	}: { ids: readonly ShortcutId[]; class?: string } = $props();

	const registry = useShortcuts();
	const apple = isApplePlatform();

	type Hint = { id: string; keys: string; label: string };

	const paired: Partial<Record<ShortcutId, { with: ShortcutId; label: string }>> = {
		"cursor-down": { with: "cursor-up", label: "Move" },
	};

	function hintOf(shortcut: Shortcut, shown: readonly ShortcutId[]): Hint {
		const pair = paired[shortcut.id as ShortcutId];

		if (pair && registry.bound(pair.with) && !shown.includes(pair.with)) {
			return {
				id: shortcut.id,
				keys: `${keycap(shortcut, apple)} ${displayKeys(shortcutOf(pair.with).keys[0], apple)}`,
				label: pair.label,
			};
		}

		return { id: shortcut.id, keys: keycap(shortcut, apple), label: shortcut.label };
	}

	const bound = $derived(ids.filter((id) => registry.bound(id)));
	const hints = $derived(
		bound.filter((id) => id !== "help").map((id) => hintOf(shortcutOf(id), bound))
	);
	const help = $derived(bound.includes("help") ? hintOf(shortcutOf("help"), bound) : null);
</script>

{#snippet hint(entry: Hint)}
	<span class="inline-flex h-4.5 shrink-0 items-center gap-1.5 text-sm whitespace-nowrap text-muted-foreground">
		<Kbd keys={entry.keys} />{entry.label}
	</span>
{/snippet}

{#if hints.length > 0 || help}
	<div class={cn("flex min-w-0 items-center gap-x-3", className)}>
		<div class="flex h-4.5 min-w-0 flex-wrap justify-end gap-x-3 overflow-hidden">
			{#each hints as entry (entry.id)}
				{@render hint(entry)}
			{/each}
		</div>
		{#if help}
			{@render hint(help)}
		{/if}
	</div>
{/if}
