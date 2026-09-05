import type { components } from "$lib/api/dashboard.gen";
import type { ActivityFeed } from "$lib/activity/activity";
import type { Issue } from "$lib/issues/issues";
import type { StateCategory } from "$lib/team/states";
import type { Team } from "$lib/team/teams";
import { totalIssues, type IssueProgress } from "$lib/issues/board";
import { calendarDate, daysBetween, onCalendarDate, onDate } from "$lib/time";

const targetSoonDays = 14;

export type Project = components["schemas"]["Project"];
export type ProjectState = components["schemas"]["ProjectState"];
export type ProjectHealth = components["schemas"]["ProjectHealth"];
export type ProjectMember = components["schemas"]["ProjectMember"];
export type ProjectStatusUpdate = components["schemas"]["ProjectStatusUpdate"];

export const projectStates: ProjectState[] = [
	"planned",
	"active",
	"paused",
	"completed",
	"cancelled",
];

export const stateLabels: Record<ProjectState, string> = {
	planned: "Planned",
	active: "In progress",
	paused: "Paused",
	completed: "Completed",
	cancelled: "Cancelled",
};

export const stateNotes: Record<ProjectState, string> = {
	planned: "Agreed, but not started.",
	active: "Work is happening now.",
	paused: "Deliberately set down, to be picked up again.",
	completed: "Finished. It can be archived.",
	cancelled: "Stopped for good. It can be archived.",
};

export const healths: ProjectHealth[] = ["on_track", "at_risk", "off_track"];

export const healthLabels: Record<ProjectHealth, string> = {
	on_track: "On track",
	at_risk: "At risk",
	off_track: "Off track",
};

export function stateLabel(state: ProjectState): string {
	return stateLabels[state];
}

export function healthLabel(health: ProjectHealth | undefined): string {
	return health ? healthLabels[health] : "No update yet";
}

export function finished(state: ProjectState): boolean {
	return state === "completed" || state === "cancelled";
}

export function projectPath(workspace: string, project: Project): string {
	return `/${workspace}/projects/${project.slug}`;
}

export function projectsPath(workspace: string): string {
	return `/${workspace}/projects`;
}

export type ProjectListing =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "empty" }
	| { kind: "ready"; projects: Project[] };

export type ProjectDetail =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			project: Project;
			members: ProjectMember[];
			updates: ProjectStatusUpdate[];
			issues: Issue[];
			nextCursor: string | undefined;
			activity: ActivityFeed;
	  };

export type ProjectFailure =
	| { kind: "slug_taken" }
	| { kind: "archived" }
	| { kind: "not_archived" }
	| { kind: "not_finished" }
	| { kind: "member_exists" }
	| { kind: "invalid"; fields: string[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

const failureMessages: Record<ProjectFailure["kind"], string> = {
	slug_taken: "Another project in this workspace already uses that address.",
	archived: "This project is archived. Bring it back before changing it.",
	not_archived: "This project is not archived.",
	not_finished: "Only a completed or cancelled project can be archived.",
	member_exists: "They are already on this project.",
	invalid: "Check the highlighted fields and try again.",
	forbidden: "Only the project lead or a workspace administrator can change this project.",
	unavailable: "Something went wrong and nothing changed. Wait a moment and try again.",
};

export function projectFailureMessage(failure: ProjectFailure): string {
	return failureMessages[failure.kind];
}

const conflictKinds: Record<string, ProjectFailure["kind"]> = {
	project_slug_taken: "slug_taken",
	project_archived: "archived",
	project_not_archived: "not_archived",
	project_not_finished: "not_finished",
	project_member_exists: "member_exists",
};

export function readProjectFailure(problem: unknown): ProjectFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	if ("code" in problem && typeof problem.code === "string") {
		const kind = conflictKinds[problem.code];
		if (kind) return { kind } as ProjectFailure;
	}

	if ("errors" in problem && Array.isArray(problem.errors)) {
		return {
			kind: "invalid",
			fields: problem.errors.map((entry: { field?: string }) => entry.field ?? ""),
		};
	}

	if ("status" in problem && problem.status === 403) return { kind: "forbidden" };

	return { kind: "unavailable" };
}

export type CategoryGroup = { category: StateCategory; issues: Issue[] };

const categoryOrder: StateCategory[] = ["not_started", "active", "complete", "abandoned"];

export function groupByCategory(issues: Issue[]): CategoryGroup[] {
	return categoryOrder
		.map((category) => ({
			category,
			issues: issues.filter((issue) => issue.state.category === category),
		}))
		.filter((group) => group.issues.length > 0);
}

export function teamProjectsPath(workspace: string, teamId: string): string {
	return `${projectsPath(workspace)}?teamId=${teamId}`;
}

export type ProjectStanding = "on_track" | "at_risk" | "completed" | "cancelled";

export function projectStanding(project: Project): ProjectStanding {
	if (project.state === "completed") return "completed";
	if (project.state === "cancelled") return "cancelled";
	if (project.health === "at_risk" || project.health === "off_track") return "at_risk";

	return "on_track";
}

export const standingLabels: Record<ProjectStanding, string> = {
	on_track: "On track",
	at_risk: "At risk",
	completed: "Completed",
	cancelled: "Cancelled",
};

export function targetLine(project: Project, now: string, timezone: string): string {
	const standing = projectStanding(project);

	if (standing === "completed") {
		return project.archivedAt
			? `completed ${onDate(project.archivedAt, timezone)}`
			: "completed";
	}

	if (standing === "cancelled") {
		return project.archivedAt
			? `cancelled ${onDate(project.archivedAt, timezone)}`
			: "cancelled";
	}

	if (!project.targetOn) return "no target date";

	const days = daysBetween(calendarDate(now, timezone), project.targetOn);
	const target = `target ${onCalendarDate(project.targetOn)}`;

	if (days > 0 && days <= targetSoonDays) {
		return `${target} · ${days} ${days === 1 ? "day" : "days"}`;
	}

	return days < 0 ? `${target} · ${-days} ${days === -1 ? "day" : "days"} over` : target;
}

export type ScopeSegment = { label: string; count: number; token: string };

export function scopeSegments(progress: IssueProgress): ScopeSegment[] {
	return [
		{ label: "Done", count: progress.complete, token: "status-complete" },
		{ label: "In progress", count: progress.active, token: "status-active" },
		{ label: "Not started", count: progress.notStarted, token: "status-not-started" },
		{ label: "Cancelled", count: progress.abandoned, token: "status-abandoned" },
	].filter((segment) => segment.count > 0);
}

export function progressLabel(progress: IssueProgress): string {
	const total = totalIssues(progress);

	if (total === 0) return "no issues yet";

	return `${progress.complete}/${total} done · ${Math.round((progress.complete / total) * 100)}%`;
}

export function teamsLine(project: Project, teams: Team[]): string {
	const named = (project.teamIds ?? [])
		.map((id) => teams.find((team) => team.id === id)?.name)
		.filter((name): name is string => Boolean(name));

	return named.length > 0 ? named.join(" · ") : "Every team";
}
