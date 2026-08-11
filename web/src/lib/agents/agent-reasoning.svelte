<script lang="ts">
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import type { AgentReasoning } from "./agents";

	let { reasoning }: { reasoning: AgentReasoning | undefined } = $props();

	const sources = $derived(reasoning?.consulted ?? []);
	const said = $derived(
		Boolean(reasoning?.observed) || Boolean(reasoning?.uncertain) || sources.length > 0
	);
</script>

{#if said}
	<div class="flex flex-col gap-2.5 rounded-md border border-line-subtle p-2.5">
		{#if reasoning?.observed}
			<div class="flex flex-col gap-0.5">
				<Eyebrow class="text-ink-600">What it saw</Eyebrow>
				<p class="text-sm leading-normal text-ink-900 text-pretty">{reasoning.observed}</p>
			</div>
		{/if}

		{#if sources.length > 0}
			<div class="flex flex-col gap-0.5">
				<Eyebrow class="text-ink-600">What it read</Eyebrow>
				<ul class="flex flex-col gap-0.5">
					{#each sources as source (source.label)}
						<li class="min-w-0 text-sm leading-normal text-ink-900">
							{#if source.url}
								<a
									href={source.url}
									target="_blank"
									rel="noreferrer"
									class="underline underline-offset-2 motion-control hover:text-ink-600"
								>
									{source.label}
								</a>
							{:else}
								{source.label}
							{/if}
						</li>
					{/each}
				</ul>
			</div>
		{/if}

		{#if reasoning?.uncertain}
			<div class="flex flex-col gap-0.5">
				<Eyebrow class="text-warning">What it is unsure about</Eyebrow>
				<p class="text-sm leading-normal text-ink-900 text-pretty">{reasoning.uncertain}</p>
			</div>
		{/if}
	</div>
{:else}
	<p class="text-xs leading-normal text-muted-foreground text-pretty">
		This agent gave no reasoning, so deciding means going and looking for yourself.
	</p>
{/if}
