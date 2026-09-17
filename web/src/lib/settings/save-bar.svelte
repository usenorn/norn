<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import { Button } from "$lib/components/ui/button/index.js";
	import { saveBarLabel, type SaveBar } from "$lib/workspace/settings";

	let {
		bar,
		formId,
		ondiscard,
	}: { bar: SaveBar; formId: string; ondiscard: () => void } = $props();

	const saving = $derived(bar.kind === "saving");
	const conflict = $derived(bar.kind === "conflict");
</script>

{#if bar.kind !== "hidden"}
	<div
		class="flex flex-none flex-wrap items-center gap-x-3 gap-y-2 border-b border-line-strong bg-paper-2 px-4 py-2 sm:px-5"
		role="status"
	>
		<span
			class={[
				"flex min-w-0 flex-1 items-center gap-1.5 text-sm",
				conflict ? "text-destructive" : saving ? "text-muted-foreground" : "text-ink-700",
			]}
		>
			{#if conflict}
				<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
			{/if}
			{saveBarLabel(bar)}
		</span>
		<div class="flex items-center gap-2">
			<Button variant="ghost" size="sm" disabled={saving} onclick={ondiscard}>Discard</Button>
			<Button type="submit" form={formId} size="sm" disabled={saving || conflict}>
				{saving ? "Saving" : "Save changes"}
			</Button>
		</div>
	</div>
{/if}
