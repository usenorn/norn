import type { IssueGroupBy, IssueSort } from "./filter";

export const issueTabs = ["active", "backlog", "all"] as const;

export type IssueTab = (typeof issueTabs)[number];

export const tabLabels: Record<IssueTab, string> = {
	active: "Active",
	backlog: "Backlog",
	all: "All issues",
};

export const issueLayouts = ["list", "board"] as const;

export type IssueLayout = (typeof issueLayouts)[number];

export const groupings = ["state", "priority", "assignee", "project", "due", "none"] as const;

export type Grouping = (typeof groupings)[number];

export const boardGroupings: Grouping[] = ["state", "priority", "assignee", "project", "none"];

export const taskGroupings: Grouping[] = ["due", "state", "priority", "project", "none"];

export const groupingLabels: Record<Grouping, string> = {
	state: "Status",
	priority: "Priority",
	assignee: "Assignee",
	project: "Project",
	due: "Due date",
	none: "No grouping",
};

export const groupingNouns: Record<Grouping, string> = {
	state: "status",
	priority: "priority",
	assignee: "assignee",
	project: "project",
	due: "due date",
	none: "none",
};

export const orderings = ["manual", "priority", "due"] as const;

export type Ordering = (typeof orderings)[number];

export const orderingLabels: Record<Ordering, string> = {
	manual: "Manual",
	priority: "Priority",
	due: "Due date",
};

export const rowProperties = ["labels", "due"] as const;

export type RowProperty = (typeof rowProperties)[number];

export const rowPropertyLabels: Record<RowProperty, string> = {
	labels: "Labels",
	due: "Due date",
};

export type Display = {
	grouping: Grouping;
	ordering: Ordering;
	shown: RowProperty[];
	showEmpty: boolean;
};

export const displaySurfaces = ["issues", "tasks"] as const;

export type DisplaySurface = (typeof displaySurfaces)[number];

export const surfaceDefaults: Record<DisplaySurface, Display> = {
	issues: { grouping: "state", ordering: "manual", shown: [...rowProperties], showEmpty: false },
	tasks: { grouping: "due", ordering: "due", shown: [...rowProperties], showEmpty: false },
};

export const surfaceGroupings: Record<DisplaySurface, Grouping[]> = {
	issues: boardGroupings,
	tasks: taskGroupings,
};

export const surfaceOrderings: Record<DisplaySurface, Ordering[]> = {
	issues: [...orderings],
	tasks: ["due", "priority"],
};

function pick<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export function readDisplay(params: URLSearchParams, defaults: Display): Display {
	const hidden = (params.get("hide") ?? "").split(",");

	return {
		grouping: pick(params.get("group"), groupings, defaults.grouping),
		ordering: pick(params.get("order"), orderings, defaults.ordering),
		shown: rowProperties.filter((property) => !hidden.includes(property)),
		showEmpty: params.get("empty") === "1",
	};
}

export function readLayout(params: URLSearchParams): IssueLayout {
	return pick(params.get("layout"), issueLayouts, "list");
}

export function readTab(params: URLSearchParams): IssueTab {
	return pick(params.get("tab"), issueTabs, "active");
}

export const displayKeys = ["group", "order", "empty", "hide", "layout", "tab"] as const;

export function carriesDisplay(params: URLSearchParams): boolean {
	return displayKeys.some((key) => params.has(key));
}

export function writeDisplay(
	display: Display,
	layout?: IssueLayout,
	tab?: IssueTab
): URLSearchParams {
	const hidden = rowProperties.filter((property) => !display.shown.includes(property));
	const params = new URLSearchParams();

	params.set("group", display.grouping);
	params.set("order", display.ordering);
	params.set("empty", display.showEmpty ? "1" : "0");
	params.set("hide", hidden.join(","));

	if (layout) params.set("layout", layout);
	if (tab) params.set("tab", tab);

	return params;
}

export function displayCookie(
	surface: DisplaySurface,
	accountId: string,
	workspaceId: string
): string {
	return `norn.${surface}.${accountId}.${workspaceId}`;
}

export function hiddenParam(shown: RowProperty[], toggled: RowProperty): string | null {
	const next = rowProperties.filter((property) =>
		property === toggled ? shown.includes(property) : !shown.includes(property)
	);

	return next.length > 0 ? next.join(",") : null;
}

export function atDefaults(display: Display, defaults: Display): boolean {
	return (
		display.grouping === defaults.grouping &&
		display.ordering === defaults.ordering &&
		display.shown.length === defaults.shown.length &&
		display.showEmpty === defaults.showEmpty
	);
}

export function groupByFor(grouping: Grouping): IssueGroupBy {
	return grouping === "none" || grouping === "due" ? "state" : grouping;
}

export function sortFor(ordering: Ordering): IssueSort[] {
	switch (ordering) {
		case "priority":
			return [{ field: "priority" }, { field: "createdAt", descending: true }];
		case "due":
			return [{ field: "dueOn" }, { field: "createdAt", descending: true }];
		default:
			return [{ field: "rank" }];
	}
}

export const orderedProperty: Partial<Record<Ordering, "priority" | "due">> = {
	priority: "priority",
	due: "due",
};
