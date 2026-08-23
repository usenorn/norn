<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { atClock, onDateAndTime } from "$lib/time";
	import { actorLine, eventLabel, eventLine, type ExecutionEvent } from "./executions";

	let {
		timeline,
		timezone,
		more,
		working,
		onmore,
	}: {
		timeline: ExecutionEvent[];
		timezone: string;
		more: boolean;
		working: boolean;
		onmore: () => void;
	} = $props();
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Timeline">
	<Eyebrow rule>Timeline</Eyebrow>

	{#if timeline.length === 0}
		<p class="py-2 text-xs text-muted-foreground">Nothing has happened yet.</p>
	{:else}
		<ol class="flex min-w-0 flex-col">
			{#each timeline as event (event.id)}
				<li class="flex min-w-0 items-baseline gap-2.5 border-b border-line-subtle py-1.5 last:border-b-0">
					<time
						class="w-12 flex-none font-mono text-2xs text-muted-foreground"
						datetime={event.occurredAt}
						title={onDateAndTime(event.occurredAt, timezone)}
					>
						{atClock(event.occurredAt, timezone)}
					</time>
					<span class="w-16 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
						{eventLabel(event.kind)}
					</span>
					<span class="min-w-0 flex-1 text-xs leading-normal break-words text-ink-900 text-pretty">
						{eventLine(event)}
					</span>
					<span class="hidden flex-none text-2xs text-muted-foreground sm:inline">
						{actorLine(event)}
					</span>
				</li>
			{/each}
		</ol>
	{/if}

	{#if more}
		<div class="pt-1">
			<Button variant="ghost" size="sm" disabled={working} onclick={onmore}>
				{working ? "Reading on…" : "Read on"}
			</Button>
		</div>
	{/if}
</section>
