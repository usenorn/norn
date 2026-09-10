<script lang="ts">
	import type { Snippet } from "svelte";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import CalendarDays from "@lucide/svelte/icons/calendar-days";
	import Folder from "@lucide/svelte/icons/folder";
	import Funnel from "@lucide/svelte/icons/funnel";
	import Tags from "@lucide/svelte/icons/tags";
	import Users from "@lucide/svelte/icons/users";
	import X from "@lucide/svelte/icons/x";
	import { Button } from "$lib/components/ui/button/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import PropertyPicker from "$lib/issues/property-picker.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import {
		chosenCount,
		clearedLink,
		facetChips,
		facetOptions,
		type FacetCatalogue,
	} from "$lib/issues/facet-options";
	import { facetLabels, type FacetKind, type Facets } from "$lib/issues/facets";
	import type { LinkWith } from "$lib/issues/linking";

	let {
		facets,
		offered,
		catalogue,
		linkWith,
		open = $bindable(false),
		actions,
	}: {
		facets: Facets;
		offered: FacetKind[];
		catalogue: FacetCatalogue;
		linkWith: LinkWith;
		open?: boolean;
		actions?: Snippet;
	} = $props();

	let category = $state<FacetKind | null>(null);
	let search = $state("");

	const options = $derived(facetOptions(offered, catalogue, facets, category, search, linkWith));
	const chips = $derived(facetChips(offered, catalogue, facets));
	const chosen = $derived(chosenCount(offered, facets));
	const cleared = $derived(clearedLink(offered, linkWith));

	$effect(() => {
		if (!open) category = null;
	});

	function pick(value: string) {
		if (value === "") {
			category = null;

			return;
		}

		if (!category && offered.includes(value as FacetKind)) {
			category = value as FacetKind;
			open = true;
		}
	}
</script>

<PropertyPicker
	{options}
	bind:open
	bind:search
	placeholder={category
		? `Filter by ${facetLabels[category].toLowerCase()}…`
		: "Filter by property or value…"}
	class="w-59"
	empty="No matching property"
	onpick={pick}
>
	{#snippet trigger(props)}
		<Button {...props} variant="ghost" size="sm" class="shrink-0">
			<Funnel aria-hidden="true" />
			Filter
		</Button>
	{/snippet}
	{#snippet shortcut()}
		<Kbd keys="F" />
	{/snippet}
	{#snippet mark(option)}
		{#if !category && search.trim() === ""}
			{#if option.value === "state"}
				<StatusIcon category="not_started" decorative />
			{:else if option.value === "assignee"}
				<Users class="text-muted-foreground" aria-hidden="true" />
			{:else if option.value === "priority"}
				<PriorityIcon priority="high" />
			{:else if option.value === "label"}
				<Tags class="text-muted-foreground" aria-hidden="true" />
			{:else if option.value === "project"}
				<Folder class="text-muted-foreground" aria-hidden="true" />
			{:else}
				<CalendarDays class="text-muted-foreground" aria-hidden="true" />
			{/if}
		{:else if option.value === ""}
			<ArrowLeft class="text-muted-foreground" aria-hidden="true" />
		{/if}
	{/snippet}
</PropertyPicker>

{#each chips as chip (chip.kind)}
	<span
		class="inline-flex shrink-0 items-center gap-1.5 border-b-2 border-line-strong pb-0.5 text-sm whitespace-nowrap text-foreground"
	>
		{chip.label}
		<a
			href={linkWith({ [chip.kind]: null })}
			aria-label="Remove the {facetLabels[chip.kind].toLowerCase()} filter"
			class="text-muted-foreground hover:text-ink-900"
		>
			<X class="size-3" aria-hidden="true" />
		</a>
	</span>
{/each}

{#if chosen > 1}
	<a href={cleared} class="shrink-0 text-sm text-muted-foreground hover:text-foreground">
		Clear all
	</a>
{/if}

{@render actions?.()}
