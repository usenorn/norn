<script lang="ts">
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import Funnel from "@lucide/svelte/icons/funnel";
	import X from "@lucide/svelte/icons/x";
	import { Button } from "#lib/components/ui/button/index.js";
	import PropertyPicker, { type PickerOption } from "#lib/issues/property-picker.svelte";
	import type { Project } from "#lib/projects/projects.js";
	import {
		memoriesPath,
		memoryFacetChips,
		memoryFacetLabels,
		memoryFacetValues,
		memoryFacetsFor,
		type MemoryFacet,
		type MemoryQuery,
	} from "./memories";

	let {
		workspace,
		query,
		projects,
		components,
	}: {
		workspace: string;
		query: MemoryQuery;
		projects: Project[];
		components: string[];
	} = $props();

	let open = $state(false);
	let search = $state("");
	let facet = $state<MemoryFacet | null>(null);

	const offered = $derived(memoryFacetsFor(query.tab));
	const chips = $derived(memoryFacetChips(workspace, query, projects));

	const options = $derived.by((): PickerOption[] => {
		if (!facet) {
			return offered.map((value) => ({ value, label: memoryFacetLabels[value], trailing: true }));
		}

		const chosen = facet;
		const values = memoryFacetValues(chosen, projects, components).map((option) => ({
			value: `${chosen}:${option.value}`,
			label: option.label,
			checked: query[chosen] === option.value,
			href: memoriesPath(workspace, { ...query, [chosen]: option.value }),
		}));
		const typed = search.trim();
		const typedComponent =
			chosen === "component" && typed && !values.some((option) => option.label === typed)
				? [
						{
							value: `component:${typed}`,
							label: `Component “${typed}”`,
							href: memoriesPath(workspace, { ...query, component: typed }),
						},
					]
				: [];

		return [{ value: "", label: "All properties" }, ...typedComponent, ...values];
	});

	$effect(() => {
		if (!open) facet = null;
	});

	function pick(value: string) {
		if (value === "") {
			facet = null;

			return;
		}

		const chosen = offered.find((candidate) => candidate === value);

		if (!facet && chosen) {
			facet = chosen;
			open = true;
		}
	}
</script>

<div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
	<PropertyPicker
		{options}
		bind:open
		bind:search
		placeholder={facet ? `Filter by ${memoryFacetLabels[facet].toLowerCase()}…` : "Filter by property…"}
		class="w-59"
		empty={facet === "component" ? "Type a component" : "No matching property"}
		closeOnPick={false}
		onpick={pick}
	>
		{#snippet trigger(props)}
			<Button {...props} variant="ghost" size="sm" class="shrink-0">
				<Funnel aria-hidden="true" />
				Filter
			</Button>
		{/snippet}
		{#snippet mark(option)}
			{#if facet && option.value === ""}
				<ArrowLeft class="text-muted-foreground" aria-hidden="true" />
			{/if}
		{/snippet}
	</PropertyPicker>

	{#each chips as chip (chip.facet)}
		<span
			class="inline-flex max-w-full min-w-0 shrink-0 items-center gap-1.5 border-b-2 border-line-strong pb-0.5 text-sm text-foreground"
		>
			<span class="min-w-0 truncate">{chip.label}</span>
			<a
				href={chip.removed}
				aria-label="Remove the {memoryFacetLabels[chip.facet].toLowerCase()} filter"
				class="text-muted-foreground hover:text-ink-900"
			>
				<X class="size-3" aria-hidden="true" />
			</a>
		</span>
	{/each}

	{#if chips.length > 1}
		<a href={memoriesPath(workspace, { tab: query.tab })} class="shrink-0 text-sm text-muted-foreground hover:text-foreground">
			Clear all
		</a>
	{/if}
</div>
