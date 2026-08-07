import type { components } from "$lib/api/dashboard.gen";
import type { StateCategory } from "$lib/team/states";
import type { IssueTab } from "./display";

export type IssueFilter = components["schemas"]["IssueFilter"];
export type IssueSort = components["schemas"]["IssueSort"];
export type IssueGroupBy = components["schemas"]["IssueGroupBy"];
export type IssueGroupTally = components["schemas"]["IssueGroupTally"];

export const issuePageSize = 50;
export const issueGroupSize = 25;

const openCategories: StateCategory[] = ["not_started", "active"];

export function tabFilter(tab: IssueTab, backlogStateIds: string[] = []): IssueFilter | undefined {
	const backlog: IssueFilter | undefined =
		backlogStateIds.length > 0
			? { field: "state", op: "in", values: backlogStateIds }
			: undefined;

	switch (tab) {
		case "backlog":
			return backlog ?? { field: "state", op: "is_not_set" };
		case "active": {
			const open: IssueFilter = { field: "stateCategory", op: "in", values: openCategories };

			return backlog ? { all: [open, { not: backlog }] } : open;
		}
		default:
			return undefined;
	}
}

export function teamFilter(teamId: string): IssueFilter {
	return { field: "team", op: "is", values: [teamId] };
}

export function allOf(...parts: (IssueFilter | undefined)[]): IssueFilter | undefined {
	const present = parts.filter((part): part is IssueFilter => part !== undefined);

	if (present.length === 0) return undefined;
	if (present.length === 1) return present[0];

	return { all: present };
}

export type IssueQueryBody = components["schemas"]["IssueQueryRequest"];

export function columnQuery(
	base: IssueQueryBody,
	within: IssueFilter | undefined,
	cursor: string | undefined
): IssueQueryBody {
	return {
		text: base.text,
		filter: allOf(base.filter, within),
		sort: base.sort,
		limit: issuePageSize,
		cursor,
	};
}

export function tallyOf(groups: IssueGroupTally[] | undefined, key: string): number | undefined {
	return groups?.find((group) => group.key === key)?.issues;
}

export function tallyTotal(groups: IssueGroupTally[] | undefined): number | undefined {
	return groups?.reduce((sum, group) => sum + group.issues, 0);
}
