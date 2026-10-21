<script lang="ts">
	import { goto } from "$app/navigation";
	import Check from "@lucide/svelte/icons/check";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import SlidersHorizontal from "@lucide/svelte/icons/sliders-horizontal";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Switch } from "$lib/components/ui/switch/index.js";
	import {
		atDefaults,
		groupingLabels,
		groupingNouns,
		hiddenParam,
		orderingLabels,
		rowProperties,
		rowPropertyLabels,
		type Display,
		type Grouping,
		type Ordering,
	} from "$lib/issues/display";
	import type { LinkWith } from "$lib/issues/linking";

	let {
		display,
		defaults,
		groupings,
		orderings,
		linkWith,
		emptyGroups = true,
	}: {
		display: Display;
		defaults: Display;
		groupings: Grouping[];
		orderings: Ordering[];
		linkWith: LinkWith;
		emptyGroups?: boolean;
	} = $props();

	let open = $state(false);

	const groupLink = $derived((grouping: Grouping) =>
		linkWith({ group: grouping === defaults.grouping ? null : grouping })
	);

	const orderLink = $derived((ordering: Ordering) =>
		linkWith({ order: ordering === defaults.ordering ? null : ordering })
	);

	const resetLink = $derived(
		linkWith({
			group: defaults.grouping,
			order: defaults.ordering,
			empty: defaults.showEmpty ? "1" : "0",
			hide: rowProperties.filter((property) => !defaults.shown.includes(property)).join(",") || null,
		})
	);
</script>

{#snippet choice(href: string, label: string, chosen: boolean)}
	<a
		{href}
		aria-current={chosen ? "true" : undefined}
		class="flex h-7 items-center gap-2 rounded-sm px-1.5 text-md text-foreground hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
	>
		<span class="flex-1">{label}</span>
		{#if chosen}
			<Check class="size-3.25 text-ink-900" aria-hidden="true" />
		{/if}
	</a>
{/snippet}

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				variant={atDefaults(display, defaults) ? "ghost" : "subtle"}
				size="sm"
				class="shrink-0"
			>
				<SlidersHorizontal aria-hidden="true" />
				Grouped by {groupingNouns[display.grouping]}
				<ChevronDown aria-hidden="true" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content align="end" class="w-69 p-1.5">
		<div class="flex flex-col">
			<span class="caption px-1.5 pt-1 pb-1 text-ink-600">Group by</span>
			{#each groupings as grouping (grouping)}
				{@render choice(groupLink(grouping), groupingLabels[grouping], display.grouping === grouping)}
			{/each}

			<span class="-mx-1.5 my-1.5 h-px bg-line-subtle" aria-hidden="true"></span>

			<span class="caption px-1.5 pt-1 pb-1 text-ink-600">Order by</span>
			{#each orderings as ordering (ordering)}
				{@render choice(orderLink(ordering), orderingLabels[ordering], display.ordering === ordering)}
			{/each}

			<span class="-mx-1.5 my-1.5 h-px bg-line-subtle" aria-hidden="true"></span>

			{#if emptyGroups}
				<label class="flex h-7.5 items-center gap-2 px-1.5 text-md text-foreground">
					<span class="flex-1">Show empty groups</span>
					<Switch
						checked={display.showEmpty}
						onCheckedChange={() =>
							goto(linkWith({ empty: display.showEmpty ? null : "1" }), { noScroll: true })}
					/>
				</label>
			{/if}
			{#each rowProperties as property (property)}
				<label class="flex h-7.5 items-center gap-2 px-1.5 text-md text-foreground">
					<span class="flex-1">Show {rowPropertyLabels[property].toLowerCase()}</span>
					<Switch
						checked={display.shown.includes(property)}
						onCheckedChange={() =>
							goto(linkWith({ hide: hiddenParam(display.shown, property) }), { noScroll: true })}
					/>
				</label>
			{/each}

			{#if !atDefaults(display, defaults)}
				<span class="-mx-1.5 my-1.5 h-px bg-line-subtle" aria-hidden="true"></span>
				<a
					href={resetLink}
					class="flex h-7 items-center rounded-sm px-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-foreground"
				>
					Reset display
				</a>
			{/if}
		</div>
	</Popover.Content>
</Popover.Root>
