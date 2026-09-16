import { kindLabels, kindOrder, type SearchGroup } from "$lib/search/search";
import {
	matches,
	type Destination,
	type PaletteCommand,
	type PaletteGroup,
	type PaletteListing,
	type PaletteMode,
	type StepOption,
} from "./model";

export type SearchState =
	| { kind: "idle" }
	| { kind: "searching" }
	| { kind: "slow"; scanning: number }
	| { kind: "ready"; groups: SearchGroup[]; fuzzy: boolean }
	| { kind: "unavailable" };

export type ListingSources = {
	mode: PaletteMode;
	search: SearchState;
	recents: Destination[];
	destinations: Destination[];
	commands: PaletteCommand[];
	options: StepOption[];
	everything: Destination | null;
};

export const suggestedLimit = 3;
export const destinationLimit = 8;

export function paletteListing(sources: ListingSources): PaletteListing {
	const { mode } = sources;

	if (mode.kind === "step") {
		return {
			kind: "step",
			step: mode.step,
			groups: [
				{
					id: "step",
					heading: mode.step.label,
					meta: `${sources.options.length} options`,
					entries: sources.options
						.filter((option) => matches(option.label, mode.query))
						.map((option) => ({ kind: "option", option })),
				},
			],
		};
	}

	if (mode.kind === "commands") {
		return {
			kind: "commands",
			groups: [
				{
					id: "commands",
					heading: "Commands",
					meta: `${sources.commands.length} available`,
					entries: sources.commands
						.filter((command) => matches(command.label, mode.query))
						.map((command) => ({ kind: "command", command })),
				},
			],
		};
	}

	const query = mode.query;
	const places = placesMatching(sources.destinations, query);

	if (sources.search.kind === "unavailable") {
		return {
			kind: "unavailable",
			groups: present([
				recentGroup(sources.recents.filter((recent) => matches(recent.label, query))),
				places,
			]),
		};
	}

	if (query === "") {
		return {
			kind: "recent",
			groups: present([
				recentGroup(sources.recents),
				{
					id: "suggested",
					heading: "Suggested",
					meta: "commands",
					entries: sources.commands
						.slice(0, suggestedLimit)
						.map((command) => ({ kind: "command", command })),
				},
				{
					id: "go-to",
					heading: "Go to",
					entries: sources.destinations
						.filter((destination) => destination.keys)
						.map((destination) => ({ kind: "destination", destination })),
				},
			]),
		};
	}

	switch (sources.search.kind) {
		case "slow":
			return { kind: "slow", groups: present([places]), scanning: sources.search.scanning };
		case "ready": {
			const groups = present([...resultGroups(sources.search.groups), places]);

			if (groups.length === 0) return { kind: "no_matches", query };

			const everything: PaletteGroup[] = sources.everything
				? [
						{
							id: "everything",
							heading: "",
							entries: [{ kind: "destination", destination: sources.everything }],
						},
					]
				: [];

			return { kind: "results", groups: [...groups, ...everything], fuzzy: sources.search.fuzzy };
		}
		default:
			return { kind: "searching", groups: present([places]) };
	}
}

function recentGroup(recents: Destination[]): PaletteGroup {
	return {
		id: "recent",
		heading: "Recent",
		meta: "from this device",
		entries: recents.map((destination) => ({ kind: "destination", destination })),
	};
}

function placesMatching(destinations: Destination[], query: string): PaletteGroup {
	return {
		id: "places",
		heading: "Go to",
		entries: destinations
			.filter((destination) => matches(`${destination.label} ${destination.context ?? ""}`, query))
			.slice(0, destinationLimit)
			.map((destination) => ({ kind: "destination", destination })),
	};
}

function resultGroups(groups: SearchGroup[]): PaletteGroup[] {
	return [...groups]
		.sort((a, b) => kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind))
		.map((group) => ({
			id: group.kind,
			heading: kindLabels[group.kind],
			meta: `${group.results.length}`,
			entries: group.results.map((result) => ({ kind: "result", result })),
		}));
}

function present(groups: PaletteGroup[]): PaletteGroup[] {
	return groups.filter((group) => group.entries.length > 0);
}
