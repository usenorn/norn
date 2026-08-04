import type { components } from "$lib/api/dashboard.gen";
import type { Issue } from "./issues";

export type { Issue };
import type { StateCategory, WorkflowState } from "$lib/team/states";

export type IssueProgress = components["schemas"]["IssueProgress"];

export const issueTabs = ["open", "closed", "all"] as const;
export type IssueTab = (typeof issueTabs)[number];

export const issueLayouts = ["list", "board"] as const;
export type IssueLayout = (typeof issueLayouts)[number];

export const tabLabels: Record<IssueTab, string> = {
	open: "Open",
	closed: "Closed",
	all: "All",
};

const tabCategories: Record<IssueTab, StateCategory[] | null> = {
	open: ["not_started", "active"],
	closed: ["complete", "abandoned"],
	all: null,
};

export type IssueBoard =
	| { kind: "loading" }
	| { kind: "no_teams" }
	| { kind: "empty"; team: string }
	| { kind: "ready"; groups: IssueGroup[] }
	| { kind: "unavailable" };

export type IssueGroup = {
	state: WorkflowState;
	issues: Issue[];
};

export function selectIssues(issues: Issue[], tab: IssueTab): Issue[] {
	const categories = tabCategories[tab];
	if (!categories) return issues;

	return issues.filter((issue) => categories.includes(issue.state.category));
}

export function groupByState(
	issues: Issue[],
	states: WorkflowState[],
	options: { showEmpty?: boolean } = {}
): IssueGroup[] {
	const groups = states.map((state) => ({
		state,
		issues: issues.filter((issue) => issue.state.id === state.id),
	}));

	return options.showEmpty ? groups : groups.filter((group) => group.issues.length > 0);
}

export function boardFor(
	issues: Issue[] | undefined,
	states: WorkflowState[] | undefined,
	tab: IssueTab,
	teamName: string,
	options: { showEmpty?: boolean } = {}
): IssueBoard {
	if (!issues || !states) return { kind: "unavailable" };

	const selected = selectIssues(issues, tab);
	if (selected.length === 0) return { kind: "empty", team: teamName };

	return { kind: "ready", groups: groupByState(selected, states, options) };
}

export function countForTab(issues: Issue[], tab: IssueTab): number {
	return selectIssues(issues, tab).length;
}

export function totalIssues(progress: IssueProgress): number {
	return progress.notStarted + progress.active + progress.complete + progress.abandoned;
}
