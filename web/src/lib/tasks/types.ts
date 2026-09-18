import type { IssuePriority } from "$lib/issues/issues";
import type { LabelColor } from "$lib/labels/labels";
import type { StateCategory } from "$lib/team/states";

export type TaskLabel = { name: string; color: LabelColor };

export type TaskState = { name: string; category: StateCategory };

export type Task = {
	id: string;
	title: string;
	state: TaskState;
	priority: IssuePriority;
	assignee: string | null;
	assigneeAccountId: string | null;
	date: string | null;
	labels: TaskLabel[];
	project: string;
	cycle: string | null;
};

export type TaskBucket = { key: string; label: string; emphasis: boolean; tasks: Task[] };
