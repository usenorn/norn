import type { Grouping } from "$lib/issues/display";
import { dueBucketLabels, dueBuckets, dueBucketOf } from "$lib/issues/facets";
import { priorities, priorityLabel, type Issue, type IssuePriority } from "$lib/issues/issues";
import type { Project } from "$lib/projects/projects";
import type { WorkflowState } from "$lib/team/states";
import { calendarDate, onCalendarDate } from "$lib/time";
import type { Task, TaskBucket } from "./types";

export type GroupingContext = { states: WorkflowState[]; projects: Project[] };

export function taskOf(issue: Issue, assignee: string | null): Task {
	return {
		id: issue.reference,
		title: issue.title,
		state: { name: issue.state.name, category: issue.state.category },
		priority: issue.priority,
		assignee,
		date: issue.dueOn ? onCalendarDate(issue.dueOn) : null,
		labels: issue.labels.map((label) => ({ name: label.name, color: label.color })),
		project: issue.projectName ?? "",
		cycle: issue.cycleNumber === undefined ? null : String(issue.cycleNumber),
	};
}

export function bucketsOf(
	issues: Issue[],
	assignee: string | null,
	now: string,
	timezone: string
): TaskBucket[] {
	const today = calendarDate(now, timezone);
	const grouped = new Map<string, Task[]>();

	for (const issue of issues) {
		const key = dueBucketOf(issue.dueOn, today);

		grouped.set(key, [...(grouped.get(key) ?? []), taskOf(issue, assignee)]);
	}

	return dueBuckets
		.filter((bucket) => grouped.has(bucket))
		.map((bucket) => ({
			key: bucket,
			label: dueBucketLabels[bucket],
			emphasis: bucket === "overdue",
			tasks: grouped.get(bucket) ?? [],
		}));
}

function slotsFor(grouping: Grouping, context: GroupingContext): { key: string; label: string }[] {
	switch (grouping) {
		case "state":
			return context.states.map((state) => ({ key: state.id, label: state.name }));
		case "priority":
			return priorities.map((entry) => ({ key: entry.value, label: entry.label }));
		case "project":
			return [
				...context.projects.map((project) => ({ key: project.id, label: project.name })),
				{ key: "", label: "No project" },
			];
		default:
			return [{ key: "", label: "All tasks" }];
	}
}

function slotOf(issue: Issue, grouping: Grouping): string {
	switch (grouping) {
		case "state":
			return issue.state.id;
		case "priority":
			return issue.priority;
		case "project":
			return issue.projectId ?? "";
		default:
			return "";
	}
}

export function groupsOf(
	issues: Issue[],
	assignee: string | null,
	grouping: Grouping,
	context: GroupingContext
): TaskBucket[] {
	const grouped = new Map<string, Task[]>();

	for (const issue of issues) {
		const key = slotOf(issue, grouping);

		grouped.set(key, [...(grouped.get(key) ?? []), taskOf(issue, assignee)]);
	}

	const named = slotsFor(grouping, context);
	const listed = named.filter((slot) => grouped.has(slot.key));
	const missing = [...grouped.keys()].filter(
		(key) => !named.some((slot) => slot.key === key)
	);

	return [
		...listed.map((slot) => ({
			key: slot.key,
			label: slot.label,
			emphasis: grouping === "priority" && slot.key === ("urgent" as IssuePriority),
			tasks: grouped.get(slot.key) ?? [],
		})),
		...missing.map((key) => ({
			key,
			label: grouping === "priority" ? priorityLabel(key as IssuePriority) : "Elsewhere",
			emphasis: false,
			tasks: grouped.get(key) ?? [],
		})),
	];
}
