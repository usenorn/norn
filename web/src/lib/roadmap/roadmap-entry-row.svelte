<script lang="ts">
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import Folder from "@lucide/svelte/icons/folder";
	import MoreHorizontal from "@lucide/svelte/icons/more-horizontal";
	import Pencil from "@lucide/svelte/icons/pencil";
	import RefreshCw from "@lucide/svelte/icons/refresh-cw";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import Pill, { type PillTone } from "$lib/components/norn/pill.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import RoadmapFieldChip from "./roadmap-field-chip.svelte";
	import { timelineLabel } from "./public-roadmap";
	import {
		effectiveFlag,
		entryBadgeLabels,
		entryBadges,
		entryFields,
		flagEffectLabels,
		visibilityLabels,
		type EntryBadge,
		type RoadmapActivity,
		type RoadmapEntry,
		type RoadmapVisibility,
	} from "./admin";

	let {
		entry,
		visibility,
		activity,
		onedit,
		onrefresh,
		onremove,
	}: {
		entry: RoadmapEntry;
		visibility: RoadmapVisibility;
		activity: RoadmapActivity;
		onedit: (entry: RoadmapEntry) => void;
		onrefresh: (entry: RoadmapEntry) => void;
		onremove: (entry: RoadmapEntry) => void;
	} = $props();

	const badgeTones: Record<EntryBadge, PillTone> = {
		not_published: "muted",
		withheld: "warning",
		out_of_reach: "warning",
	};

	const busy = $derived(activity.kind !== "idle");
	const refreshing = $derived(activity.kind === "refreshing" && activity.entryId === entry.id);
	const removing = $derived(activity.kind === "removing" && activity.entryId === entry.id);
	const badges = $derived(entryBadges(entry));
	const fields = $derived(
		entryFields(entry.kind).map((field) => ({ field, effect: effectiveFlag(visibility, entry.visibility, field) }))
	);
	const datesShown = $derived(effectiveFlag(visibility, entry.visibility, "timeline") === "shown");
	const dates = $derived(timelineLabel(entry.timeline));
</script>

<div class="flex flex-col gap-2 px-3 py-3">
	<div class="flex items-start gap-2">
		{#if entry.kind === "project"}
			<Folder class="mt-0.5 size-icon-row shrink-0 text-muted-foreground" aria-label="Project" />
		{:else}
			<CircleDot class="mt-0.5 size-icon-row shrink-0 text-muted-foreground" aria-label="Issue" />
		{/if}

		<div class="flex min-w-0 flex-1 flex-col gap-1">
			<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
				<span class="min-w-0 text-md break-words text-ink-900">{entry.publicTitle}</span>
				{#each badges as badge (badge)}
					<Pill tone={badgeTones[badge]}>{entryBadgeLabels[badge]}</Pill>
				{/each}
			</div>

			<p class="text-xs leading-normal break-words text-muted-foreground">
				{#if entry.source}
					Internal:
					{#if entry.source.reference}
						<span class="font-mono">{entry.source.reference}</span>
					{/if}
					{entry.source.label}
				{:else}
					The project or issue behind this entry is out of your reach.
				{/if}
			</p>
		</div>

		<div class="flex shrink-0 items-center gap-1">
			<Button size="sm" variant="ghost" disabled={busy} onclick={() => onedit(entry)}>
				<Pencil aria-hidden="true" />
				<span class="sr-only sm:not-sr-only">Edit</span>
				<span class="sr-only">{entry.publicTitle}</span>
			</Button>
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} size="icon-sm" variant="ghost" disabled={busy} aria-label="More for {entry.publicTitle}">
							<MoreHorizontal aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					<DropdownMenu.Item disabled={!entry.reachable} onSelect={() => onrefresh(entry)}>
						<RefreshCw aria-hidden="true" />
						Refresh status and date
					</DropdownMenu.Item>
					<DropdownMenu.Item variant="destructive" onSelect={() => onremove(entry)}>
						<Trash2 aria-hidden="true" />
						Remove from roadmap
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	</div>

	<div class="flex flex-wrap items-center gap-1.5">
		{#each fields as { field, effect } (field)}
			<RoadmapFieldChip
				label={visibilityLabels[field]}
				tone={effect === "shown" ? "shown" : "hidden"}
				note={flagEffectLabels[effect]}
			/>
		{/each}
	</div>

	<p class="font-mono text-xs text-muted-foreground tabular-nums" aria-live="polite">
		{#if refreshing}
			Refreshing from the source…
		{:else if removing}
			Removing…
		{:else if datesShown && dates}
			Readers see: {dates}
		{:else if datesShown}
			No dates set, so none are shown
		{:else}
			Dates are not shown
		{/if}
	</p>
</div>
