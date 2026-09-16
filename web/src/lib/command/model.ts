import type { IconComponent } from "$lib/utils.js";
import type { LabelColor } from "$lib/labels/labels";
import type { SearchResult } from "$lib/search/search";
import type { StateCategory } from "$lib/team/states";

export type IssueTarget = { id: string; reference: string; title: string; teamId: string };

export type CommandScope = { kind: "none" } | { kind: "issues"; issues: IssueTarget[] };

export type StepKind = "assign" | "status" | "cycle" | "label";

export type CommandStep = { kind: StepKind; label: string };

export type StepOption = {
	value: string;
	label: string;
	hint?: string;
	icon?: IconComponent;
	category?: StateCategory;
	dot?: LabelColor;
	person?: { accountId: string; name: string };
};

export type Destination = {
	id: string;
	label: string;
	href: string;
	context?: string;
	keys?: string;
	icon?: IconComponent;
};

export type CommandId =
	| "issue-new"
	| "assign"
	| "status"
	| "cycle"
	| "label"
	| "copy-link"
	| "open-triage"
	| "toggle-density";

export type PaletteCommand = {
	id: CommandId;
	label: string;
	keys?: string;
	icon?: IconComponent;
	step?: StepKind;
};

export type PaletteEntry =
	| { kind: "destination"; destination: Destination }
	| { kind: "command"; command: PaletteCommand }
	| { kind: "result"; result: SearchResult }
	| { kind: "option"; option: StepOption };

export type PaletteGroup = {
	id: string;
	heading: string;
	meta?: string;
	entries: PaletteEntry[];
};

export type PaletteListing =
	| { kind: "recent"; groups: PaletteGroup[] }
	| { kind: "searching"; groups: PaletteGroup[] }
	| { kind: "results"; groups: PaletteGroup[]; fuzzy: boolean }
	| { kind: "no_matches"; query: string }
	| { kind: "slow"; groups: PaletteGroup[]; scanning: number }
	| { kind: "commands"; groups: PaletteGroup[] }
	| { kind: "step"; step: CommandStep; groups: PaletteGroup[] }
	| { kind: "unavailable"; groups: PaletteGroup[] };

export type CommandRun =
	| { kind: "idle" }
	| { kind: "running"; entry: string }
	| { kind: "failed"; title: string; detail: string };

export type RecentEntry = { id: string; href: string };

export const commandPrefix = ">";
export const createQueryLimit = 32;

export const searchPlaceholder = "Search issues, projects, people and views…";
export const commandPlaceholder = "Run a command…";
export const stepPlaceholder = "Filter…";

export function placeholderFor(listing: PaletteListing): string {
	if (listing.kind === "step") return stepPlaceholder;
	if (listing.kind === "commands") return commandPlaceholder;

	return searchPlaceholder;
}

export function groupsOf(listing: PaletteListing): PaletteGroup[] {
	return listing.kind === "no_matches" ? [] : listing.groups;
}

export function entriesOf(listing: PaletteListing): PaletteEntry[] {
	return groupsOf(listing).flatMap((group) => group.entries);
}

export function entryCount(listing: PaletteListing): number {
	return entriesOf(listing).length;
}

export function entryId(entry: PaletteEntry): string {
	switch (entry.kind) {
		case "destination":
			return entry.destination.id;
		case "command":
			return entry.command.id;
		case "option":
			return entry.option.value;
		default:
			return entry.result.id;
	}
}

export function entryLabel(entry: PaletteEntry): string {
	switch (entry.kind) {
		case "destination":
			return entry.destination.label;
		case "command":
			return entry.command.label;
		case "option":
			return entry.option.label;
		default:
			return entry.result.title;
	}
}

export function matches(haystack: string, query: string): boolean {
	return haystack.toLowerCase().includes(query.trim().toLowerCase());
}

export function scopedIssues(scope: CommandScope): IssueTarget[] {
	return scope.kind === "issues" ? scope.issues : [];
}

export function scopeSubject(scope: CommandScope): string {
	const issues = scopedIssues(scope);

	if (issues.length === 1) return issues[0].reference;

	return `${issues.length} issues`;
}

export function sharedTeam(scope: CommandScope): string | null {
	const teams = new Set(scopedIssues(scope).map((issue) => issue.teamId));

	return teams.size === 1 ? [...teams][0] : null;
}

export type PaletteMode =
	| { kind: "search"; query: string }
	| { kind: "commands"; query: string }
	| { kind: "step"; step: CommandStep; query: string };

export function modeOf(typed: string, step: CommandStep | null): PaletteMode {
	if (step) return { kind: "step", step, query: typed.trim() };

	if (typed.startsWith(commandPrefix)) {
		return { kind: "commands", query: typed.slice(commandPrefix.length).trim() };
	}

	return { kind: "search", query: typed.trim() };
}

export function enterHint(listing: PaletteListing): string {
	if (listing.kind === "step") return "apply";
	if (listing.kind === "commands") return "run";

	return "open";
}

export function footNote(listing: PaletteListing, scope: CommandScope): string {
	if (listing.kind === "unavailable") return "degraded · local only";
	if ((listing.kind === "commands" || listing.kind === "step") && scope.kind === "issues") {
		return `commands act on ${scopeSubject(scope)}`;
	}

	const count = entryCount(listing);

	return count === 1 ? "1 result" : `${count} results`;
}

export function createLabel(query: string): string {
	const shown = query.length > createQueryLimit ? `${query.slice(0, createQueryLimit)}…` : query;

	return `Create issue “${shown}”`;
}
