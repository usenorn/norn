<script lang="ts">
	import type { ImportCount, ImportCountTone } from "$lib/import/types";

	let { counts }: { counts: ImportCount[] } = $props();

	const valueTone: Record<ImportCountTone, string> = {
		normal: "text-ink-900",
		warning: "text-warning",
		danger: "text-destructive",
		muted: "text-muted-foreground",
	};
</script>

<dl
	class="flex flex-wrap gap-px overflow-hidden rounded-lg border border-line-subtle bg-line-subtle"
>
	{#each counts as count (count.label)}
		<div class="flex min-w-32 flex-[1_1_150px] flex-col gap-1 bg-card p-3.5">
			<dd class="font-mono text-3xl tracking-tighter tabular-nums {valueTone[count.tone]}">
				{count.value}
			</dd>
			<dt class="font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
				{count.label}
			</dt>
		</div>
	{/each}
</dl>
