<script lang="ts">
	import { goto } from "$app/navigation";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Check from "@lucide/svelte/icons/check";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import Settings from "@lucide/svelte/icons/settings";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
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
	let pane = $state<"root" | "grouping" | "ordering">("root");

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

	$effect(() => {
		if (!open) pane = "root";
	});
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="ghost" size="sm" class="shrink-0">
				Grouped by {groupingNouns[display.grouping]}
				<ChevronDown aria-hidden="true" />
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end">
		<DropdownMenu.Label>Group by</DropdownMenu.Label>
		{#each groupings as grouping (grouping)}
			<DropdownMenu.Item>
				{#snippet child({ props })}
					<a href={groupLink(grouping)} {...props}>
						<span class="flex-1">{groupingLabels[grouping]}</span>
						{#if display.grouping === grouping}
							<span class="font-mono text-2xs text-ink-600">✓</span>
						{/if}
					</a>
				{/snippet}
			</DropdownMenu.Item>
		{/each}
	</DropdownMenu.Content>
</DropdownMenu.Root>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				variant="outline"
				size="icon-sm"
				aria-label="Display options"
				class={atDefaults(display, defaults)
					? ""
					: "border-primary bg-primary text-primary-foreground hover:border-primary-active hover:bg-primary-active hover:text-primary-foreground"}
			>
				<Settings class="size-icon-toolbar" aria-hidden="true" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content align="end" class="w-69">
		{#if pane === "root"}
			<div class="flex flex-col p-3">
				<button
					type="button"
					onclick={() => (pane = "grouping")}
					class="flex h-7.5 cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left hover:bg-accent"
				>
					<span class="text-md text-foreground">Grouping</span>
					<span class="flex-1"></span>
					<span class="text-sm text-muted-foreground">{groupingLabels[display.grouping]}</span>
					<ChevronRight class="size-3.25 text-muted-foreground" aria-hidden="true" />
				</button>

				<button
					type="button"
					onclick={() => (pane = "ordering")}
					class="flex h-7.5 cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left hover:bg-accent"
				>
					<span class="text-md text-foreground">Ordering</span>
					<span class="flex-1"></span>
					<span class="text-sm text-muted-foreground">{orderingLabels[display.ordering]}</span>
					<ChevronRight class="size-3.25 text-muted-foreground" aria-hidden="true" />
				</button>

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

				<span class="-mx-3 my-2.5 h-px bg-line-subtle" aria-hidden="true"></span>

				<span class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
					Display properties
				</span>
				<div class="flex flex-wrap gap-1.5 pt-2">
					{#each rowProperties as property (property)}
						<a
							href={linkWith({ hide: hiddenParam(display.shown, property) })}
							data-on={display.shown.includes(property)}
							class="inline-flex h-5.5 items-center rounded-sm border border-line-default bg-card px-2 text-xs font-medium text-ink-600 motion-control hover:text-ink-900 data-[on=true]:border-primary data-[on=true]:bg-primary data-[on=true]:text-primary-foreground"
						>
							{rowPropertyLabels[property]}
						</a>
					{/each}
				</div>

				<span class="-mx-3 mt-3 mb-2 h-px bg-line-subtle" aria-hidden="true"></span>

				<a href={resetLink} class="text-sm text-muted-foreground hover:text-foreground">
					Reset display
				</a>
			</div>
		{:else}
			<div class="flex flex-col p-1.5">
				<button
					type="button"
					onclick={() => (pane = "root")}
					class="flex h-7 cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left hover:bg-accent"
				>
					<ArrowLeft class="size-3.25 text-muted-foreground" aria-hidden="true" />
					<span class="text-sm text-muted-foreground">
						{pane === "grouping" ? "Grouping" : "Ordering"}
					</span>
				</button>

				<span class="-mx-1.5 my-1 h-px bg-line-subtle" aria-hidden="true"></span>

				{#if pane === "grouping"}
					{#each groupings as grouping (grouping)}
						<a
							href={groupLink(grouping)}
							class="flex h-7 items-center gap-2 rounded-sm px-1.5 text-md text-foreground hover:bg-accent"
						>
							<span class="flex-1">{groupingLabels[grouping]}</span>
							{#if display.grouping === grouping}
								<Check class="size-3.25 text-ink-900" aria-hidden="true" />
							{/if}
						</a>
					{/each}
				{:else}
					{#each orderings as ordering (ordering)}
						<a
							href={orderLink(ordering)}
							class="flex h-7 items-center gap-2 rounded-sm px-1.5 text-md text-foreground hover:bg-accent"
						>
							<span class="flex-1">{orderingLabels[ordering]}</span>
							{#if display.ordering === ordering}
								<Check class="size-3.25 text-ink-900" aria-hidden="true" />
							{/if}
						</a>
					{/each}
				{/if}
			</div>
		{/if}
	</Popover.Content>
</Popover.Root>
