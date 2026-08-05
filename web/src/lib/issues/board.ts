import type { components } from "$lib/api/dashboard.gen";
import type { Issue, IssuePriority } from "./issues";

export type { Issue };
import type { WorkflowState } from "$lib/team/states";
import type { Project } from "$lib/projects/projects";
import { tallyOf, type IssueGroupTally } from "./filter";
import { priorities } from "./issues";
import type { Grouping } from "./display";

export type IssueProgress = components["schemas"]["IssueProgress"];

export const issueTabs = ["active", "backlog", "all"] as const;
export type IssueTab = (typeof issueTabs)[number];

export const issueLayouts = ["list", "board"] as const;
export type IssueLayout = (typeof issueLayouts)[number];

export const tabLabels: Record<IssueTab, string> = {
	active: "Active",
	backlog: "Backlog",
	all: "All issues",
};

export function backlogStates(states: WorkflowState[]): WorkflowState[] {
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
	if (!tallies) return undefined;

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
	| { kind: "loading" }
	| { kind: "no_teams" }
	| { kind: "empty"; team: string }
	| { kind: "ready"; columns: IssueColumn[] }
	| { kind: "unavailable" };

export type IssueGroup = {
	state: WorkflowState;
	issues: Issue[];
	total: number;
};

export function groupByState(
	issues: Issue[],
	states: WorkflowState[],
	tallies: IssueGroupTally[] | undefined,
	options: { showEmpty?: boolean } = {}
): IssueGroup[] {
	const known = new Set(states.map((state) => state.id));
	const loose = issues.filter((issue) => !known.has(issue.state.id));

	const groups = states.map((state) => {
		const held = issues.filter((issue) => issue.state.id === state.id);

		return { state, issues: held, total: tallyOf(tallies, state.id) ?? held.length };
	});

	for (const issue of loose) {
		const existing = groups.find((group) => group.state.id === issue.state.id);

		if (existing) {
			existing.issues.push(issue);

			continue;
		}

		groups.push({
			state: { ...issue.state, teamId: issue.teamId, isDefault: false, isCompletion: false },
			issues: [issue],
			total: tallyOf(tallies, issue.state.id) ?? 1,
		});
	}

	return options.showEmpty ? groups : groups.filter((group) => group.issues.length > 0);
}

export function statesInPlay(issues: Issue[]): WorkflowState[] {
	const seen = new Map<string, WorkflowState>();

	for (const issue of issues) {
		if (seen.has(issue.state.id)) continue;

		seen.set(issue.state.id, {
			...issue.state,
			teamId: issue.teamId,
			isDefault: false,
			isCompletion: false,
		});
	}

	return [...seen.values()].sort(
		(a, b) => a.position - b.position || a.name.localeCompare(b.name)
	);
}

export function boardFor(
	issues: Issue[] | undefined,
	grouping: Grouping,
	context: GroupingContext,
	tallies: IssueGroupTally[] | undefined,
	scopeName: string,
	options: { showEmpty?: boolean } = {}
): IssueBoard {
	if (!issues) return { kind: "unavailable" };

	if (issues.length === 0) return { kind: "empty", team: scopeName };

	return { kind: "ready", columns: columnsFor(issues, grouping, context, tallies, options) };
}

export type GroupMark =
	| { kind: "state"; state: WorkflowState }
	| { kind: "priority"; priority: IssuePriority }
	| { kind: "assignee"; name: string }
	| { kind: "project" }
	| { kind: "all" };

export type IssueColumn = {
	key: string;
	name: string;
	mark: GroupMark;
	issues: Issue[];
	total: number;
};

export type GroupingContext = {
	states: WorkflowState[];
	members: { accountId: string; displayName?: string }[];
	projects: Project[];
};

function stateColumns(issues: Issue[], context: GroupingContext): IssueColumn[] {
	const known = context.states.length > 0 ? context.states : statesInPlay(issues);

	return known.map((state) => ({
		key: state.id,
		name: state.name,
		mark: { kind: "state", state } as const,
		issues: issues.filter((issue) => issue.state.id === state.id),
		total: 0,
	}));
}

function priorityColumns(issues: Issue[]): IssueColumn[] {
	return priorities.map((priority) => ({
		key: priority.value,
		name: priority.label,
		mark: { kind: "priority", priority: priority.value } as const,
		issues: issues.filter((issue) => issue.priority === priority.value),
		total: 0,
	}));
}

function assigneeColumns(issues: Issue[], context: GroupingContext): IssueColumn[] {
	const named = context.members.map((member) => ({
		key: member.accountId,
		name: member.displayName ?? "Someone",
		mark: { kind: "assignee", name: member.displayName ?? "" } as const,
		issues: issues.filter((issue) => issue.assigneeAccountId === member.accountId),
		total: 0,
	}));

	return [
		...named,
		{
			key: "",
			name: "Unassigned",
			mark: { kind: "assignee", name: "" } as const,
			issues: issues.filter((issue) => !issue.assigneeAccountId),
			total: 0,
		},
	];
}

function projectColumns(issues: Issue[], context: GroupingContext): IssueColumn[] {
	const named = context.projects.map((project) => ({
		key: project.id,
		name: project.name,
		mark: { kind: "project" } as const,
		issues: issues.filter((issue) => issue.projectId === project.id),
		total: 0,
	}));

	return [
		...named,
		{
			key: "",
			name: "No project",
			mark: { kind: "project" } as const,
			issues: issues.filter((issue) => !issue.projectId),
			total: 0,
		},
	];
}

export function columnsFor(
	issues: Issue[],
	grouping: Grouping,
	context: GroupingContext,
	tallies: IssueGroupTally[] | undefined,
	options: { showEmpty?: boolean } = {}
): IssueColumn[] {
	const columns: IssueColumn[] =
		grouping === "priority"
			? priorityColumns(issues)
			: grouping === "assignee"
				? assigneeColumns(issues, context)
				: grouping === "project"
					? projectColumns(issues, context)
					: grouping === "none"
						? [{ key: "all", name: "All issues", mark: { kind: "all" }, issues, total: 0 }]
						: stateColumns(issues, context);

	const placed = new Set(columns.flatMap((column) => column.issues.map((issue) => issue.id)));
	const loose = issues.filter((issue) => !placed.has(issue.id));

	if (loose.length > 0 && grouping === "state") {
		for (const issue of loose) {
			const existing = columns.find((column) => column.key === issue.state.id);

			if (existing) {
				existing.issues.push(issue);

				continue;
			}

			columns.push({
				key: issue.state.id,
				name: issue.state.name,
				mark: {
					kind: "state",
					state: { ...issue.state, teamId: issue.teamId, isDefault: false, isCompletion: false },
				},
				issues: [issue],
				total: 0,
			});
		}
	}

	const counted = columns.map((column) => ({
		...column,
		total: tallyOf(tallies, column.key) ?? column.issues.length,
	}));

	return options.showEmpty ? counted : counted.filter((column) => column.issues.length > 0);
}

export function totalIssues(progress: IssueProgress): number {
	return progress.notStarted + progress.active + progress.complete + progress.abandoned;
}
