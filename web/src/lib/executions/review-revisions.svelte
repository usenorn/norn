<script lang="ts">
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { onDateAndTime } from "$lib/time";
	import { revisionLabel, type ReviewRevision } from "./review";

	let {
		revisions,
		current,
		latest,
		timezone,
	}: {
		revisions: ReviewRevision[];
		current: number;
		latest: number;
		timezone: string;
	} = $props();
</script>

<nav class="flex min-w-0 flex-col gap-1.5" aria-label="Review revisions">
	<Eyebrow rule>Revisions</Eyebrow>
	<ol class="flex min-w-0 flex-wrap gap-1.5">
		{#each revisions as held (held.revision)}
			<li>
				<Button
					variant="chip"
					size="chip"
					href={held.revision === latest ? "?" : `?revision=${held.revision}`}
					aria-current={held.revision === current ? "page" : undefined}
					title={onDateAndTime(held.reportedAt, timezone)}
				>
					{revisionLabel(held.revision)}
					<span class="text-success">+{held.additions}</span>
					<span class="text-destructive">−{held.deletions}</span>
				</Button>
			</li>
		{/each}
	</ol>
</nav>
