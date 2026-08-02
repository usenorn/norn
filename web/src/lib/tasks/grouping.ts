import type { Task, TaskPriority, TaskStatus } from "./types";
import { priorityLabels, statusLabels } from "./types";

export const issueTabs = ["active", "backlog", "all"] as const;
export type IssueTab = (typeof issueTabs)[number];

export const issueLayouts = ["list", "board"] as const;
export type IssueLayout = (typeof issueLayouts)[number];

export const groupKeys = ["status", "priority", "assignee", "project", "none"] as const;
export type GroupKey = (typeof groupKeys)[number];

export const orderKeys = ["manual", "priority", "due"] as const;
export type OrderKey = (typeof orderKeys)[number];

export const groupLabels: Record<GroupKey, string> = {
	status: "status",
	priority: "priority",
	assignee: "assignee",
	project: "project",
	none: "none",
};

export const orderLabels: Record<OrderKey, string> = {
	manual: "Manual",
	priority: "Priority",
	due: "Due date",
};

export type IssueFilters = {
	status?: TaskStatus;
	priority?: TaskPriority;
	assignee?: string;
	label?: string;
	project?: string;
	cycle?: string;
};

export const filterLabels: Record<keyof IssueFilters, string> = {
	status: "Status",
	priority: "Priority",
	assignee: "Assignee",
	label: "Label",
	project: "Project",
	cycle: "Cycle",
};

const tabPredicate: Record<IssueTab, (task: Task) => boolean> = {
	active: (task) => task.status === "started" || task.status === "review" || task.status === "todo",
	backlog: (task) => task.status === "backlog",
	all: () => true,
};

export function matchesFilters(task: Task, filters: IssueFilters): boolean {
	if (filters.status && task.status !== filters.status) return false;
	if (filters.priority && task.priority !== filters.priority) return false;
	if (filters.assignee && (task.assignee ?? "") !== filters.assignee) return false;
	if (filters.label && !task.labels.some((label) => label.name === filters.label)) return false;
	if (filters.project && task.project.toLowerCase() !== filters.project.toLowerCase()) return false;
	if (filters.cycle && task.cycle !== filters.cycle) return false;
	return true;
}

export function selectIssues(tasks: Task[], tab: IssueTab, filters: IssueFilters): Task[] {
	return tasks.filter((task) => tabPredicate[tab](task) && matchesFilters(task, filters));
}

export function countForTab(tasks: Task[], tab: IssueTab, filters: IssueFilters): number {
	return selectIssues(tasks, tab, filters).length;
}

const priorityRank: Record<TaskPriority, number> = {
	urgent: 0,
	high: 1,
	medium: 2,
	low: 3,
	none: 4,
};

const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

function dueRank(date: string | null): number {
	if (!date) return Number.MAX_SAFE_INTEGER;
	const [month, day] = date.split(" ");
	return months.indexOf(month) * 31 + Number(day || 0);
}

export function order(tasks: Task[], by: OrderKey): Task[] {
	if (by === "priority") {
		return [...tasks].sort((a, b) => priorityRank[a.priority] - priorityRank[b.priority]);
	}
	if (by === "due") return [...tasks].sort((a, b) => dueRank(a.date) - dueRank(b.date));
	return tasks;
}

export type IssueGroup = {
	key: string;
	label: string;
	status?: TaskStatus;
	priority?: TaskPriority;
	dot?: string;
	tasks: Task[];
};

const statusOrder: TaskStatus[] = ["started", "review", "todo", "backlog", "done", "canceled"];
const boardStatusOrder: TaskStatus[] = ["backlog", "todo", "started", "review", "done"];
const priorityOrder: TaskPriority[] = ["urgent", "high", "medium", "low", "none"];

export function groupIssues(
	tasks: Task[],
	by: GroupKey,
	options: {
		layout?: IssueLayout;
		showEmpty?: boolean;
		people?: string[];
		projects?: { name: string; color: string }[];
	} = {}
): IssueGroup[] {
	const { layout = "list", showEmpty = false, people = [], projects = [] } = options;
	let groups: IssueGroup[];

	if (by === "status") {
		const keys = layout === "board" ? boardStatusOrder : statusOrder;
		groups = keys.map((status) => ({
			key: status,
			label: statusLabels[status],
			status,
			tasks: tasks.filter((task) => task.status === status),
		}));
	} else if (by === "priority") {
		groups = priorityOrder.map((priority) => ({
			key: priority,
			label: priorityLabels[priority],
			priority,
			tasks: tasks.filter((task) => task.priority === priority),
		}));
	} else if (by === "assignee") {
		groups = [...people, ""].map((person) => ({
			key: person || "unassigned",
			label: person || "Unassigned",
			tasks: tasks.filter((task) => (task.assignee ?? "") === person),
		}));
	} else if (by === "project") {
		groups = projects.map((project) => ({
			key: project.name,
			label: project.name,
			dot: project.color,
			tasks: tasks.filter((task) => task.project === project.name),
		}));
	} else {
		groups = [{ key: "all", label: "All issues", tasks }];
	}

	return showEmpty ? groups : groups.filter((group) => group.tasks.length > 0);
}
