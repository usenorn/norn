import type { Issue } from "$lib/issues/issues";
import { calendarDate, onCalendarDate, shiftDays } from "$lib/time";
import type { Task, TaskBucket } from "./types";

const thisWeekDays = 7;

const order = [
	{ key: "overdue", label: "Overdue", emphasis: true },
	{ key: "today", label: "Today", emphasis: false },
	{ key: "week", label: "This week", emphasis: false },
	{ key: "later", label: "Later", emphasis: false },
	{ key: "none", label: "No due date", emphasis: false },
];

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

function bucketOf(due: string | undefined, today: string): string {
	if (!due) return "none";
	if (due < today) return "overdue";
	if (due === today) return "today";

	return due <= shiftDays(today, thisWeekDays) ? "week" : "later";
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
		const key = bucketOf(issue.dueOn, today);

		grouped.set(key, [...(grouped.get(key) ?? []), taskOf(issue, assignee)]);
	}

	return order
		.filter((bucket) => grouped.has(bucket.key))
		.map((bucket) => ({ ...bucket, tasks: grouped.get(bucket.key) ?? [] }));
}
