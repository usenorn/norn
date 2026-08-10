<script lang="ts">
	import Kbd from "$lib/components/norn/kbd.svelte";
	import { useShortcuts } from "../registry.svelte";
	import { roamMode } from "./shortcuts";

	const shortcuts = useShortcuts();

	const roaming = $derived(shortcuts.mode === roamMode);
</script>

{#if roaming}
	<span
		class="inline-flex items-center gap-1.5 text-xs text-ink-900"
		role="status"
		aria-live="polite"
	>
		<Kbd keys="W A S D" />moving — <Kbd keys="Esc" />to stop
	</span>
{/if}
