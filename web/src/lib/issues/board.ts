import type { components } from "$lib/api/dashboard.gen";
import type { Issue } from "./issues";

export type { Issue };
import type { WorkflowState } from "$lib/team/states";
import { tallyOf, type IssueGroupTally } from "./filter";

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

export type IssueBoard =
	| { kind: "loading" }
	| { kind: "no_teams" }
	| { kind: "empty"; team: string }
	| { kind: "ready"; groups: IssueGroup[] }
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

export function boardFor(
	issues: Issue[] | undefined,
	states: WorkflowState[] | undefined,
	tallies: IssueGroupTally[] | undefined,
	teamName: string,
	options: { showEmpty?: boolean } = {}
): IssueBoard {
	if (!issues || !states) return { kind: "unavailable" };

	if (issues.length === 0) return { kind: "empty", team: teamName };

	return { kind: "ready", groups: groupByState(issues, states, tallies, options) };
}

export function countForTab(progress: IssueProgress, tab: IssueTab): number {
	switch (tab) {
		case "open":
			return progress.notStarted + progress.active;
		case "closed":
			return progress.complete + progress.abandoned;
		default:
			return totalIssues(progress);
	}
}

export function totalIssues(progress: IssueProgress): number {
	return progress.notStarted + progress.active + progress.complete + progress.abandoned;
}
