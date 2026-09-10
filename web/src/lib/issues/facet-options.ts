import type { PickerOption } from "./property-picker.svelte";
import { dueWindowLabels, dueWindows, facetLabels, unassigned, type FacetKind, type Facets } from "./facets";
import type { LinkWith } from "./linking";

export type FacetEntry = { value: string; label: string };

export type FacetCatalogue = Partial<Record<FacetKind, FacetEntry[]>>;

export type FacetChip = { kind: FacetKind; label: string };

export function dueEntries(): FacetEntry[] {
	return dueWindows.map((window) => ({ value: window, label: dueWindowLabels[window] }));
}

export function unassignedEntry(): FacetEntry {
	return { value: unassigned, label: "Unassigned" };
}

function entriesFor(kind: FacetKind, catalogue: FacetCatalogue): FacetEntry[] {
	return catalogue[kind] ?? [];
}

export function facetValues(
	kind: FacetKind,
	catalogue: FacetCatalogue,
	facets: Facets
): PickerOption[] {
	return entriesFor(kind, catalogue).map((entry) => ({
		value: entry.value,
		label: entry.label,
		checked: facets[kind] === entry.value,
	}));
}

export function facetOptions(
	offered: FacetKind[],
	catalogue: FacetCatalogue,
	facets: Facets,
	category: FacetKind | null,
	search: string,
	linkWith: LinkWith
): PickerOption[] {
	if (category) {
		return [
			{ value: "", label: "All properties" },
			...facetValues(category, catalogue, facets).map((option) => ({
				...option,
				href: linkWith({ [category]: option.checked ? null : option.value }),
			})),
		];
	}

	if (search.trim() === "") {
		return offered.map((kind) => ({ value: kind, label: facetLabels[kind], trailing: true }));
	}

	return offered.flatMap((kind) =>
		facetValues(kind, catalogue, facets).map((option) => ({
			...option,
			value: `${kind}:${option.value}`,
			label: `${facetLabels[kind]} · ${option.label}`,
			href: linkWith({ [kind]: option.checked ? null : option.value }),
		}))
	);
}

export function facetChips(
	offered: FacetKind[],
	catalogue: FacetCatalogue,
	facets: Facets
): FacetChip[] {
	return offered
		.filter((kind) => Boolean(facets[kind]))
		.map((kind) => {
			const value = facets[kind] as string;
			const named =
				entriesFor(kind, catalogue).find((entry) => entry.value === value)?.label ??
				`Unknown ${facetLabels[kind].toLowerCase()}`;

			return { kind, label: `${facetLabels[kind]}: ${named}` };
		});
}

export function clearedLink(offered: FacetKind[], linkWith: LinkWith): string {
	return linkWith(Object.fromEntries(offered.map((kind) => [kind, null])));
}

export function chosenCount(offered: FacetKind[], facets: Facets): number {
	return offered.filter((kind) => facets[kind] !== undefined).length;
}
