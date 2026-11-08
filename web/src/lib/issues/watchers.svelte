<script lang="ts">
	import PersonAvatar from "#lib/components/norn/person-avatar.svelte";
	import PersonDetails from "#lib/components/norn/person-details.svelte";
	import PersonTooltip from "#lib/components/norn/person-tooltip.svelte";
	import * as Popover from "#lib/components/ui/popover/index.js";
	import type { PersonDetail } from "#lib/workspace/members.js";

	const avatarLimit = 5;

	let { watchers }: { watchers: PersonDetail[] } = $props();

	const shown = $derived(watchers.slice(0, avatarLimit));
	const remaining = $derived(watchers.slice(avatarLimit));
</script>

{#if watchers.length > 0}
	<span class="flex min-w-0 flex-1 -space-x-1.5">
		{#each shown as watcher (watcher.accountId)}
			<PersonTooltip person={watcher}>
				<PersonAvatar
					accountId={watcher.accountId}
					name={watcher.name}
					size="xs"
					class="ring-1 ring-card"
				/>
			</PersonTooltip>
		{/each}
		{#if remaining.length > 0}
			<Popover.Root>
				<Popover.Trigger
					openOnHover
					openDelay={0}
					aria-label="Show {remaining.length} more watching"
					class="relative inline-flex size-icon-row cursor-pointer items-center justify-center rounded-full bg-paper-2 tabular-nums text-2xs text-muted-foreground ring-1 ring-card focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
				>
					+{remaining.length}
				</Popover.Trigger>
				<Popover.Content align="end" size="body" class="w-[min(20rem,calc(100vw-2rem))]">
					<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
					<ul tabindex="0" class="flex max-h-72 flex-col gap-3 overflow-y-auto outline-hidden">
						{#each remaining as watcher (watcher.accountId)}
							<li><PersonDetails person={watcher} /></li>
						{/each}
					</ul>
				</Popover.Content>
			</Popover.Root>
		{/if}
	</span>
{:else}
	<span class="flex-1 truncate text-md text-muted-foreground">Nobody yet</span>
{/if}
