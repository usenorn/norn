import type { IssueFilter, IssueQueryBody } from "$lib/issues/filter";
import { allOf, issueGroupSize, issuePageSize, tabFilter, teamFilter } from "$lib/issues/filter";
import {
	groupByFor,
	sortFor,
	type Grouping,
	type IssueTab,
	type Ordering,
} from "$lib/issues/display";
import type { Team } from "$lib/team/teams";
import type { SavedView, ViewReference } from "./views";

export type AppliedView =
	| { kind: "none" }
	| { kind: "gone"; id: string }
	| { kind: "applied"; view: SavedView; team: Team | null; references: ViewReference[] };

export function appliedView(
	requestedId: string,
	views: SavedView[],
	teams: Team[]
): AppliedView {
	if (!requestedId) return { kind: "none" };

	const view = views.find((candidate) => candidate.id === requestedId);
	if (!view) return { kind: "gone", id: requestedId };

	return { kind: "applied", view, team: soleTeam(view, teams), references: [] };
}

function soleTeam(view: SavedView, teams: Team[]): Team | null {
	const named = namedIn(view);

	if (named.size !== 1) return null;

	const [id] = [...named];

	return teams.find((team) => team.id === id) ?? null;
}

function namedIn(view: SavedView): Set<string> {
	const named = new Set<string>();

	collectTeams(view.filter, named);

	return named;
}

export function viewTeams(applied: AppliedView, teams: Team[]): Team[] {
	if (applied.kind !== "applied") return [];

	const named = namedIn(applied.view);

	return teams.filter((team) => named.has(team.id));
}

function collectTeams(filter: SavedView["filter"] | undefined, into: Set<string>) {
	if (!filter) return;

	if (filter.not) collectTeams(filter.not, into);

	for (const branch of [...(filter.all ?? []), ...(filter.any ?? [])]) {
		collectTeams(branch, into);
	}

	if (filter.field === "team") {
		for (const value of filter.values ?? []) into.add(value);
	}
}

export type QueryShaping = {
	filters?: IssueFilter[];
	sort?: Ordering;
	grouping?: Grouping;
	backlogStateIds?: string[];
};

function sliced(grouping: Grouping): Pick<IssueQueryBody, "limit" | "perGroup"> {
	return grouping === "none" ? { limit: issuePageSize } : { perGroup: issueGroupSize };
}

export function viewQuery(
	applied: AppliedView,
	tab: IssueTab,
	teamId: string | null,
	text = "",
	shaping: QueryShaping = {}
): IssueQueryBody {
	const extra = shaping.filters ?? [];
	const chosen = shaping.sort ? sortFor(shaping.sort) : undefined;

	if (applied.kind !== "applied") {
		return {
			text: text || undefined,
			filter: allOf(teamId ? teamFilter(teamId) : undefined, tabFilter(tab, shaping.backlogStateIds), ...extra),
			sort: chosen,
			groupBy: groupByFor(shaping.grouping ?? "state"),
			...sliced(shaping.grouping ?? "state"),
		};
	}

	return {
		text: text || undefined,
		filter: allOf(applied.view.filter, tabFilter(tab, shaping.backlogStateIds), ...extra),
		sort: chosen ?? (applied.view.sort.length > 0 ? applied.view.sort : undefined),
		groupBy: shaping.grouping
			? groupByFor(shaping.grouping)
			: (applied.view.groupBy ?? "state"),
		...sliced(shaping.grouping ?? "state"),
	};
}

export function brokenIn(applied: AppliedView): ViewReference[] {
	if (applied.kind !== "applied") return [];

	return applied.references.filter((reference) => reference.state === "missing");
}
