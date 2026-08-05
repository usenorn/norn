import { onCalendarDate, shiftDays } from "$lib/time";
import type { IssueFilter } from "./filter";
import type { IssuePriority } from "./issues";

export const facetKinds = [
	"state",
	"assignee",
	"priority",
	"label",
	"project",
	"due",
	"cycle",
] as const;

export type FacetKind = (typeof facetKinds)[number];

export type Facets = Partial<Record<FacetKind, string>>;

export const facetLabels: Record<FacetKind, string> = {
	state: "Status",
	assignee: "Assignee",
	priority: "Priority",
	label: "Label",
	project: "Project",
	due: "Due",
	cycle: "Cycle",
};

export const pickableFacets: FacetKind[] = [
	"state",
	"assignee",
	"priority",
	"label",
	"project",
	"due",
];

export const unassigned = "none";

export const dueWindows = ["overdue", "today", "week", "later"] as const;

export type DueWindow = (typeof dueWindows)[number];

export const dueWindowLabels: Record<DueWindow, string> = {
	overdue: "Overdue",
	today: "Today",
	week: "This week",
	later: "Later",
};

export type DuePreset = { label: string; value: string; hint: string };

export function duePresets(today: string): DuePreset[] {
	const weekday = new Date(`${today}T00:00:00Z`).getUTCDay();
	const friday = shiftDays(today, (5 - weekday + 7) % 7 || 7);
	const tomorrow = shiftDays(today, 1);
	const nextWeek = shiftDays(today, 7);

	return [
		{ label: "Today", value: today, hint: onCalendarDate(today) },
		{ label: "Tomorrow", value: tomorrow, hint: onCalendarDate(tomorrow) },
		{ label: "This Friday", value: friday, hint: onCalendarDate(friday) },
		{ label: "Next week", value: nextWeek, hint: onCalendarDate(nextWeek) },
		{ label: "No due date", value: "", hint: "" },
	];
}

export function readFacets(params: URLSearchParams): Facets {
	const held: Facets = {};

	for (const kind of facetKinds) {
		const value = params.get(kind);
		if (value) held[kind] = value;
	}

	return held;
}

export function facetCount(facets: Facets): number {
	return facetKinds.filter((kind) => facets[kind] !== undefined).length;
}

function dueFilter(window: string, today: string): IssueFilter | undefined {
	const weekEnd = shiftDays(today, 7);

	switch (window) {
		case "overdue":
			return { field: "dueOn", op: "before", values: [today] };
		case "today":
			return { field: "dueOn", op: "on", values: [today] };
		case "week":
			return { field: "dueOn", op: "before", values: [weekEnd] };
		case "later":
			return { field: "dueOn", op: "after", values: [shiftDays(weekEnd, -1)] };
		default:
			return undefined;
	}
}

export function facetFilters(facets: Facets, today: string): IssueFilter[] {
	const parts: IssueFilter[] = [];

	if (facets.state) parts.push({ field: "state", op: "is", values: [facets.state] });

	if (facets.assignee) {
		parts.push(
			facets.assignee === unassigned
				? { field: "assignee", op: "is_not_set" }
				: { field: "assignee", op: "is", values: [facets.assignee] }
		);
	}

	if (facets.priority) {
		parts.push({ field: "priority", op: "is", values: [facets.priority as IssuePriority] });
	}

	if (facets.label) parts.push({ field: "label", op: "has_any", values: [facets.label] });
	if (facets.project) parts.push({ field: "project", op: "is", values: [facets.project] });
	if (facets.cycle) parts.push({ field: "cycle", op: "is", values: [facets.cycle] });

	if (facets.due) {
		const window = dueFilter(facets.due, today);
		if (window) parts.push(window);
	}

	return parts;
}
