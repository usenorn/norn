export const taskStatuses = ["backlog", "todo", "started", "review", "done", "canceled"] as const;
export type TaskStatus = (typeof taskStatuses)[number];

export const taskPriorities = ["urgent", "high", "medium", "low", "none"] as const;
export type TaskPriority = (typeof taskPriorities)[number];

export const labelColors = ["neutral", "cyan", "blue", "violet", "orchid", "magenta"] as const;
export type LabelColor = (typeof labelColors)[number];

export type TaskLabel = { name: string; color: LabelColor };

export type Task = {
	id: string;
	title: string;
	status: TaskStatus;
	priority: TaskPriority;
	assignee: string | null;
	date: string | null;
	labels: TaskLabel[];
	project: string;
	cycle: string | null;
};

export const statusLabels: Record<TaskStatus, string> = {
	backlog: "Backlog",
	todo: "Todo",
	started: "In progress",
	review: "In review",
	done: "Done",
	canceled: "Canceled",
};

export const priorityLabels: Record<TaskPriority, string> = {
	urgent: "Urgent",
	high: "High",
	medium: "Medium",
	low: "Low",
	none: "No priority",
};
