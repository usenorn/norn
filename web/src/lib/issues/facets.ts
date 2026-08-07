import { onCalendarDate, shiftDays } from "$lib/time";
import type { Grouping } from "./display";
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

export function propertyFilter(
	kind: FacetKind,
	value: string,
	today: string
): IssueFilter | undefined {
	switch (kind) {
		case "state":
			return value ? { field: "state", op: "is", values: [value] } : undefined;
		case "assignee":
			return value && value !== unassigned
				? { field: "assignee", op: "is", values: [value] }
				: { field: "assignee", op: "is_not_set" };
		case "priority":
			return value ? { field: "priority", op: "is", values: [value as IssuePriority] } : undefined;
		case "label":
			return value ? { field: "label", op: "has_any", values: [value] } : undefined;
		case "project":
			return value
				? { field: "project", op: "is", values: [value] }
				: { field: "project", op: "is_not_set" };
		case "cycle":
			return value ? { field: "cycle", op: "is", values: [value] } : undefined;
		default:
			return value ? dueFilter(value, today) : undefined;
	}
}

export function columnFilter(grouping: Grouping, key: string): IssueFilter | undefined {
	return grouping === "none" ? undefined : propertyFilter(grouping, key, "");
}

export function facetFilters(facets: Facets, today: string): IssueFilter[] {
	const parts: IssueFilter[] = [];

	for (const kind of facetKinds) {
		const chosen = facets[kind];
		if (chosen === undefined) continue;

		const part = propertyFilter(kind, chosen, today);
		if (part) parts.push(part);
	}

	return parts;
}
