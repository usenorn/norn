<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import Cog from "@lucide/svelte/icons/cog";
	import Plug from "@lucide/svelte/icons/plug";
	import { Button } from "$lib/components/ui/button/index.js";
	import {
		actorKindLabels,
		actorLabel,
		changeLine,
		readable,
		type ActivityEvent,
		type ActivityFeed,
	} from "$lib/activity/activity";

	let {
		feed,
		when,
		working = false,
		hideComments = false,
		emptyLine = "Nothing has happened yet.",
		onmore,
	}: {
		feed: ActivityFeed;
		when: (instant: string) => string;
		working?: boolean;
		hideComments?: boolean;
		emptyLine?: string;
		onmore: () => void;
	} = $props();

	const ready = $derived(feed.kind === "ready" ? feed : null);
	const shown = $derived(
		(ready?.events ?? []).filter((event) => !hideComments || readable(event))
	);

	function marker(event: ActivityEvent) {
		if (event.actorKind === "agent") return Bot;
		if (event.actorKind === "token") return Plug;
		if (event.actorKind === "system") return Cog;

		return null;
	}
</script>

{#if feed.kind === "loading"}
	<div class="h-16 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
{:else if feed.kind === "unavailable"}
	<p class="text-sm text-muted-foreground">We could not read this history.</p>
{:else if feed.kind === "empty" || shown.length === 0}
	<p class="text-sm text-muted-foreground">{emptyLine}</p>
{:else}
	<ol class="flex flex-col gap-3">
		{#each shown as event (event.id)}
			{@const Marker = marker(event)}
			<li class="flex flex-col gap-1 text-sm">
				<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class="flex items-center gap-1 font-medium text-ink-900">
						{#if Marker}
							<Marker
								class="size-3.5 shrink-0 text-muted-foreground"
								aria-label={actorKindLabels[event.actorKind]}
							/>
						{/if}
						{actorLabel(event)}
					</span>
					<span class="font-mono text-xs text-muted-foreground">
						<time datetime={event.createdAt}>{when(event.createdAt)}</time>
					</span>
					{#if event.bulkActionId}
						<span class="font-mono text-2xs tracking-eyebrow text-ink-600 uppercase">
							In bulk
						</span>
					{/if}
				</div>

				{#if event.changes.length === 1}
					<span class="text-ink-900">{changeLine(event.changes[0])}</span>
				{:else}
					<ul class="flex flex-col gap-0.5 border-l border-line-subtle pl-3">
						{#each event.changes as change (change.id)}
							<li class="text-ink-900">{changeLine(change)}</li>
						{/each}
					</ul>
				{/if}
			</li>
		{/each}
	</ol>

	{#if ready?.nextCursor}
		<div>
			<Button variant="secondary" size="sm" disabled={working} onclick={onmore}>
				{working ? "Loading" : "Load more history"}
			</Button>
		</div>
	{/if}
{/if}
