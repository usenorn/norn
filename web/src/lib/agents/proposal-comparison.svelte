<script lang="ts">
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import Markdown from "#lib/issues/markdown.svelte";

	let {
		now,
		proposed,
		empty,
	}: {
		now: string;
		proposed: string;
		empty: string;
	} = $props();
</script>

<div class="grid gap-2 sm:grid-cols-2">
	<div class="flex min-w-0 flex-col gap-0.5">
		<Eyebrow class="text-ink-600">Now</Eyebrow>
		<div class="max-h-48 min-w-0 overflow-y-auto rounded-sm bg-paper-2 p-2">
			{#if now}
				<Markdown source={now} />
			{:else}
				<span class="text-sm text-muted-foreground">{empty}</span>
			{/if}
		</div>
	</div>
	<div class="flex min-w-0 flex-col gap-0.5">
		<Eyebrow class="text-ink-600">Proposed</Eyebrow>
		<div class="max-h-48 min-w-0 overflow-y-auto rounded-sm bg-paper-2 p-2">
			<Markdown source={proposed} />
		</div>
	</div>
</div>
