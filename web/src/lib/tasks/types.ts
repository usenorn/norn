import type { LabelColor } from "$lib/labels/labels";
import type { StateCategory } from "$lib/team/states";

export const taskPriorities = ["urgent", "high", "medium", "low", "none"] as const;
export type TaskPriority = (typeof taskPriorities)[number];

export type TaskLabel = { name: string; color: LabelColor };

export type TaskState = { name: string; category: StateCategory };

export type Task = {
	id: string;
	title: string;
	state: TaskState;
	priority: TaskPriority;
	assignee: string | null;
	date: string | null;
	labels: TaskLabel[];
	project: string;
	cycle: string | null;
};

export const priorityLabels: Record<TaskPriority, string> = {
	urgent: "Urgent",
	high: "High",
	medium: "Medium",
	low: "Low",
	none: "No priority",
};
