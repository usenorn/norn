import type { IssueGroupBy, IssueSort } from "./filter";

export const groupings = ["state", "priority", "assignee", "project", "none"] as const;

export type Grouping = (typeof groupings)[number];

export const groupingLabels: Record<Grouping, string> = {
	state: "Status",
	priority: "Priority",
	assignee: "Assignee",
	project: "Project",
	none: "No grouping",
};

export const groupingNouns: Record<Grouping, string> = {
	state: "status",
	priority: "priority",
	assignee: "assignee",
	project: "project",
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

export const defaultDisplay: Display = {
	grouping: "state",
	ordering: "manual",
	shown: [...rowProperties],
	showEmpty: false,
};

function pick<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
	return allowed.includes(value as T) ? (value as T) : fallback;
}

export function readDisplay(params: URLSearchParams): Display {
	const hidden = (params.get("hide") ?? "").split(",");

	return {
		grouping: pick(params.get("group"), groupings, defaultDisplay.grouping),
		ordering: pick(params.get("order"), orderings, defaultDisplay.ordering),
		shown: rowProperties.filter((property) => !hidden.includes(property)),
		showEmpty: params.get("empty") === "1",
	};
}

export const displayKeys = ["group", "order", "empty", "hide", "layout"] as const;

export function carriesDisplay(params: URLSearchParams): boolean {
	return displayKeys.some((key) => params.has(key));
}

export function writeDisplay(display: Display, layout: string): URLSearchParams {
	const hidden = rowProperties.filter((property) => !display.shown.includes(property));
	const params = new URLSearchParams();

	params.set("group", display.grouping);
	params.set("order", display.ordering);
	params.set("empty", display.showEmpty ? "1" : "0");
	params.set("hide", hidden.join(","));
	params.set("layout", layout);

	return params;
}

export function displayCookie(workspaceId: string): string {
	return `norn.issues.${workspaceId}`;
}

export function hiddenParam(shown: RowProperty[], toggled: RowProperty): string | null {
	const next = rowProperties.filter((property) =>
		property === toggled ? shown.includes(property) : !shown.includes(property)
	);

	return next.length > 0 ? next.join(",") : null;
}

export function atDefaults(display: Display): boolean {
	return (
		display.grouping === defaultDisplay.grouping &&
		display.ordering === defaultDisplay.ordering &&
		display.shown.length === rowProperties.length &&
		!display.showEmpty
	);
}

export function groupByFor(grouping: Grouping): IssueGroupBy {
	return grouping === "none" ? "state" : grouping;
}

export function sortFor(ordering: Ordering): IssueSort[] | undefined {
	switch (ordering) {
		case "priority":
			return [{ field: "priority" }, { field: "createdAt", descending: true }];
		case "due":
			return [{ field: "dueOn" }, { field: "createdAt", descending: true }];
		default:
			return undefined;
	}
}
