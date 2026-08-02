import {
	groupKeys,
	issueLayouts,
	issueTabs,
	orderKeys,
	type GroupKey,
	type IssueFilters,
	type IssueLayout,
	type IssueTab,
	type OrderKey,
} from "$lib/tasks/grouping";
import type { Task } from "$lib/tasks/types";
import { issuesFixture } from "./preview";
import type { PageLoad } from "./$types";

function pick<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export const load: PageLoad = ({
	url,
}): {
	issues: Task[];
	tab: IssueTab;
	layout: IssueLayout;
	group: GroupKey;
	order: OrderKey;
	filters: IssueFilters;
	showEmpty: boolean;
} => {
	const q = url.searchParams;
	const filters: IssueFilters = {};
	for (const key of ["status", "priority", "assignee", "label", "project", "cycle"] as const) {
		const value = q.get(key);
		if (value) Object.assign(filters, { [key]: value });
	}

	return {
		issues: issuesFixture,
		tab: pick(q.get("tab"), issueTabs, "active"),
		layout: pick(q.get("layout"), issueLayouts, "list"),
		group: pick(q.get("group"), groupKeys, "status"),
		order: pick(q.get("order"), orderKeys, "manual"),
		filters,
		showEmpty: q.get("empty") === "1",
	};
};
