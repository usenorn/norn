import type { components } from "$lib/api/dashboard.gen";
import type { WorkflowState } from "$lib/team/states";
import type { Project } from "$lib/projects/projects";
import type { Issue, IssuePriority } from "./issues";
import { tabAdmits, tallyOf, tallyTotal, type IssueGroupTally } from "./filter";
import { priorities } from "./issues";
import { loadFor, pageOf, type ColumnLoad, type ColumnPage } from "./paging";
import { applyMoves, keyOf, type PendingMove } from "./drop";
import { edited, type PendingEdit } from "./pending";
import type { Grouping, IssueTab } from "./display";

export type { Issue };

export type IssueProgress = components["schemas"]["IssueProgress"];

export function backlogStates(states: WorkflowState[]): WorkflowState[] {
	const teams = new Map<string, WorkflowState[]>();

	for (const state of states) {
		teams.set(state.teamId, [...(teams.get(state.teamId) ?? []), state]);
	}

	return [...teams.values()].flatMap(teamBacklog);
}

function teamBacklog(states: WorkflowState[]): WorkflowState[] {
	const lands = states.find((state) => state.isDefault);

	if (!lands) return [];

	return states.filter(
		(state) => state.category === "not_started" && state.position < lands.position
	);
}

export function tabCounts(
	tallies: IssueGroupTally[] | undefined,
	states: WorkflowState[],
	backlog: WorkflowState[]
): Record<IssueTab, number> | undefined {
	if (!tallies || states.length === 0) return undefined;

	const category = new Map(states.map((state) => [state.id, state.category]));
	const held = new Set(backlog.map((state) => state.id));

	const waiting = backlog.reduce((sum, state) => sum + (tallyOf(tallies, state.id) ?? 0), 0);

	const open = tallies
		.filter((tally) => {
			const kind = category.get(tally.key);

			return !held.has(tally.key) && (kind === "not_started" || kind === "active");
		})
		.reduce((sum, tally) => sum + tally.issues, 0);

	return {
		active: open,
		backlog: waiting,
		all: tallies.reduce((sum, tally) => sum + tally.issues, 0),
	};
}

export type IssueBoard =
	| { kind: "no_teams" }
	| { kind: "empty"; team: string }
	| { kind: "ready"; columns: IssueColumn[] }
	| { kind: "unavailable" };

export type IssueGroup = {
	state: WorkflowState;
	issues: Issue[];
};

export function groupByState(issues: Issue[], states: WorkflowState[]): IssueGroup[] {
	const known = new Map(states.map((state) => [state.id, state]));

	for (const issue of issues) {
		if (!known.has(issue.state.id)) known.set(issue.state.id, statedBy(issue));
	}

	return [...known.values()]
		.sort((a, b) => a.position - b.position || a.name.localeCompare(b.name))
		.map((state) => ({
			state,
			issues: issues.filter((issue) => issue.state.id === state.id),
		}))
		.filter((group) => group.issues.length > 0);
}

function statedBy(issue: Issue): WorkflowState {
	return { ...issue.state, teamId: issue.teamId, isDefault: false, isCompletion: false };
}

export type ColumnSource = {
	issues: Issue[];
	tallies: IssueGroupTally[] | undefined;
	nextCursor: string | undefined;
};

export type GroupMark =
	| { kind: "state"; state: WorkflowState }
	| { kind: "priority"; priority: IssuePriority }
	| { kind: "assignee"; name: string }
	| { kind: "project" }
	| { kind: "all" }
	| { kind: "unknown" };

export type IssueColumn = {
	key: string;
	name: string;
	mark: GroupMark;
	issues: Issue[];
	total: number;
	load: ColumnLoad;
};

export type GroupingContext = {
	states: WorkflowState[];
	members: { accountId: string; displayName?: string }[];
	projects: Project[];
	tab: IssueTab;
	backlogStateIds: string[];
};

type ColumnSlot = { key: string; name: string; mark: GroupMark };

export type BoardOptions = {
	showEmpty?: boolean;
	moves?: PendingMove[];
	edits?: PendingEdit[];
	creations?: Issue[];
};

export const unknownNames: Record<Grouping, string> = {
	state: "Unknown status",
	priority: "Unknown priority",
	assignee: "Unknown person",
	project: "Unknown project",
	none: "All issues",
};

function slotsFor(grouping: Grouping, context: GroupingContext): ColumnSlot[] {
	switch (grouping) {
		case "priority":
			return priorities.map((priority) => ({
				key: priority.value,
				name: priority.label,
				mark: { kind: "priority", priority: priority.value },
			}));
		case "assignee":
			return [
				...context.members.map((member) => ({
					key: member.accountId,
					name: member.displayName ?? "Someone",
					mark: { kind: "assignee", name: member.displayName ?? "" } as const,
				})),
				{ key: "", name: "Unassigned", mark: { kind: "assignee", name: "" } },
			];
		case "project":
			return [
				...context.projects.map((project) => ({
					key: project.id,
					name: project.name,
					mark: { kind: "project" } as const,
				})),
				{ key: "", name: "No project", mark: { kind: "project" } },
			];
		case "none":
			return [{ key: "all", name: "All issues", mark: { kind: "all" } }];
		default:
			return context.states
				.filter((state) => tabAdmits(context.tab, state, context.backlogStateIds))
				.map((state) => ({
					key: state.id,
					name: state.name,
					mark: { kind: "state", state },
				}));
	}
}

function slotFrom(grouping: Grouping, key: string, issue: Issue | undefined): ColumnSlot {
	if (!issue) return { key, name: unknownNames[grouping], mark: { kind: "unknown" } };

	switch (grouping) {
		case "priority":
			return {
				key,
				name: priorities.find((priority) => priority.value === key)?.label ?? unknownNames[grouping],
				mark: { kind: "priority", priority: issue.priority },
			};
		case "project":
			return {
				key,
				name: issue.projectName || unknownNames[grouping],
				mark: { kind: "project" },
			};
		case "state":
			return { key, name: issue.state.name, mark: { kind: "state", state: statedBy(issue) } };
		default:
			return { key, name: unknownNames[grouping], mark: { kind: "unknown" } };
	}
}

function slotted(
	grouping: Grouping,
	context: GroupingContext,
	issues: Issue[],
	tallies: IssueGroupTally[] | undefined
): ColumnSlot[] {
	const slots = slotsFor(grouping, context);
	const known = new Set(slots.map((slot) => slot.key));
	const first = new Map<string, Issue>();

	for (const issue of issues) {
		const key = keyOf(grouping, issue);
		if (!first.has(key)) first.set(key, issue);
	}

	const missing = [...new Set([...(tallies ?? []).map((tally) => tally.key), ...first.keys()])]
		.filter((key) => !known.has(key))
		.map((key) => slotFrom(grouping, key, first.get(key)));

	if (grouping === "state") {
		missing.sort((a, b) => positionOf(a) - positionOf(b) || a.name.localeCompare(b.name));
	}

	return [...slots, ...missing];
}

function positionOf(slot: ColumnSlot): number {
	return slot.mark.kind === "state" ? slot.mark.state.position : Number.MAX_SAFE_INTEGER;
}

export function columnsFor(
	source: ColumnSource,
	grouping: Grouping,
	context: GroupingContext,
	pages: Record<string, ColumnPage>,
	options: BoardOptions = {}
): IssueColumn[] {
	const edits = options.edits ?? [];
	const creations = options.creations ?? [];
	const loaded = new Map<string, Issue[]>();
	const known = new Map<string, Issue>();
	const pristine = new Map<string, Issue>();

	const bucket = (issue: Issue) => {
		const held = edited(issue, edits);
		const key = keyOf(grouping, held);

		loaded.set(key, [...(loaded.get(key) ?? []), held]);
		known.set(held.id, held);
		pristine.set(issue.id, issue);
	};

	for (const issue of creations) bucket(issue);

	for (const issue of source.issues) bucket(issue);

	for (const page of Object.values(pages)) {
		for (const issue of page.issues) bucket(issue);
	}

	for (const [key, issues] of loaded) loaded.set(key, deduped(issues));

	const moves = options.moves ?? [];
	const held = applyMoves(loaded, moves, known);
	const shifted = tallyShift(moves, edits, creations, pristine, grouping);

	const columns = slotted(
		grouping,
		context,
		[...creations, ...source.issues],
		source.tallies
	).map((slot) => {
		const issues = held.get(slot.key) ?? [];
		const page = pageOf(pages, slot.key);
		const counted =
			grouping === "none" ? tallyTotal(source.tallies) : tallyOf(source.tallies, slot.key);
		const total = Math.max(
			(counted ?? issues.length) + (shifted.get(slot.key) ?? 0),
			issues.length
		);
		const cursor =
			page?.cursor ??
			(grouping === "none"
				? source.nextCursor
				: source.tallies?.find((tally) => tally.key === slot.key)?.nextCursor);

		return {
			...slot,
			issues,
			total,
			load: loadFor(issues.length, total, cursor, page?.paging ?? { kind: "idle" }),
		};
	});

	return options.showEmpty
		? columns
		: columns.filter((column) => column.total > 0 || column.issues.length > 0);
}

function tallyShift(
	moves: PendingMove[],
	edits: PendingEdit[],
	creations: Issue[],
	pristine: Map<string, Issue>,
	grouping: Grouping
): Map<string, number> {
	const shifted = new Map<string, number>();

	const shift = (from: string, to: string) => {
		if (from === to) return;

		shifted.set(from, (shifted.get(from) ?? 0) - 1);
		shifted.set(to, (shifted.get(to) ?? 0) + 1);
	};

	for (const issue of creations) {
		const key = keyOf(grouping, issue);

		shifted.set(key, (shifted.get(key) ?? 0) + 1);
	}

	for (const edit of edits) {
		const issue = pristine.get(edit.issueId);
		if (!issue) continue;

		shift(keyOf(grouping, issue), keyOf(grouping, edited(issue, edits)));
	}

	for (const move of moves) {
		const issue = pristine.get(move.issueId);
		if (!issue) continue;

		shift(keyOf(grouping, edited(issue, edits)), move.key);
	}

	return shifted;
}

function deduped(issues: Issue[]): Issue[] {
	const seen = new Set<string>();

	return issues.filter((issue) => {
		if (seen.has(issue.id)) return false;

		seen.add(issue.id);

		return true;
	});
}

export function boardFor(
	source: ColumnSource | undefined,
	grouping: Grouping,
	context: GroupingContext,
	pages: Record<string, ColumnPage>,
	scope: { name: string; teams: number },
	options: BoardOptions = {}
): IssueBoard {
	if (scope.teams === 0) return { kind: "no_teams" };

	if (!source) return { kind: "unavailable" };

	if (
		source.issues.length === 0 &&
		(tallyTotal(source.tallies) ?? 0) === 0 &&
		(options.creations ?? []).length === 0
	) {
		return { kind: "empty", team: scope.name };
	}

	return { kind: "ready", columns: columnsFor(source, grouping, context, pages, options) };
}

export function totalIssues(progress: IssueProgress): number {
	return progress.notStarted + progress.active + progress.complete + progress.abandoned;
}
