import type { IssuePriority } from "#lib/issues/issues.js";
import type { LabelColor } from "#lib/labels/labels.js";
import type { StateCategory } from "#lib/team/states.js";

export type TaskLabel = { name: string; color: LabelColor };

export type TaskState = { name: string; category: StateCategory };

export type Task = {
	id: string;
	title: string;
	state: TaskState;
	priority: IssuePriority;
	assignee: string | null;
	assigneeAccountId: string | null;
	dueOn: string | null;
	labels: TaskLabel[];
	project: string;
	cycle: string | null;
};

export type TaskBucket = { key: string; label: string; emphasis: boolean; tasks: Task[] };
