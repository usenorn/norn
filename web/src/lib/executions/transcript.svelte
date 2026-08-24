<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { atClock } from "$lib/time";
	import {
		transcriptSpeaker,
		transcriptText,
		type ExecutionTranscriptEntry,
	} from "./executions";

	let {
		transcript,
		timezone,
		more,
		working,
		onmore,
	}: {
		transcript: ExecutionTranscriptEntry[];
		timezone: string;
		more: boolean;
		working: boolean;
		onmore: () => void;
	} = $props();

	let shown = $state(60);

	const visible = $derived(transcript.slice(0, shown));
	const hidden = $derived(Math.max(0, transcript.length - visible.length));
</script>

<section class="flex min-w-0 flex-col gap-2" aria-label="Transcript">
	<Eyebrow rule>Transcript</Eyebrow>

	{#if transcript.length === 0}
		<p class="py-2 text-xs text-muted-foreground">
			Nothing yet. A transcript arrives once the coding agent has said something, and is empty
			where the workspace keeps summaries only or the retention window has passed.
		</p>
	{:else}
		<ol class="flex min-w-0 flex-col">
			{#each visible as entry, index (index)}
				<li class="flex min-w-0 flex-col gap-0.5 border-b border-line-subtle py-1.5 last:border-b-0">
					<div class="flex min-w-0 items-baseline gap-2">
						<span class="font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
							{transcriptSpeaker(entry)}
						</span>
						{#if entry.at}
							<time class="font-mono text-2xs text-muted-foreground" datetime={entry.at}>
								{atClock(entry.at, timezone)}
							</time>
						{/if}
					</div>
					<p class="min-w-0 text-xs leading-normal break-words whitespace-pre-wrap text-ink-900 text-pretty">
						{transcriptText(entry)}
					</p>
				</li>
			{/each}
		</ol>
	{/if}

	{#if hidden > 0}
		<div class="pt-1">
			<Button variant="ghost" size="sm" onclick={() => (shown += 60)}>
				Show {Math.min(60, hidden)} more of {hidden}
			</Button>
		</div>
	{:else if more}
		<div class="pt-1">
			<Button variant="ghost" size="sm" disabled={working} onclick={onmore}>
				{working ? "Reading on…" : "Read on"}
			</Button>
		</div>
	{/if}
</section>
