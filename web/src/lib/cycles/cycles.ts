import type { components } from "$lib/api/dashboard.gen";
import type { Issue } from "$lib/issues/issues";

export type Cycle = components["schemas"]["Cycle"];
export type CyclePhase = components["schemas"]["CyclePhase"];
export type CycleRollover = components["schemas"]["CycleRollover"];
export type CycleCadence = components["schemas"]["CycleCadence"];
export type CycleScopeChange = components["schemas"]["CycleScopeChange"];
export type CycleScope = components["schemas"]["CycleScope"];
export type TeamCycle = components["schemas"]["TeamCycle"];

export const cycleLengths = [1, 2, 3, 4] as const;

export const weekdays: { value: number; label: string }[] = [
	{ value: 1, label: "Monday" },
	{ value: 2, label: "Tuesday" },
	{ value: 3, label: "Wednesday" },
	{ value: 4, label: "Thursday" },
	{ value: 5, label: "Friday" },
	{ value: 6, label: "Saturday" },
	{ value: 0, label: "Sunday" },
];

export function weekdayLabel(weekday: number): string {
	return weekdays.find((day) => day.value === weekday)?.label ?? "Monday";
}

export function lengthLabel(weeks: number): string {
	return weeks === 1 ? "1 week" : `${weeks} weeks`;
}

const phaseLabels: Record<CyclePhase, string> = {
	upcoming: "Upcoming",
	current: "In progress",
	ended: "Needs closing",
	closed: "Closed",
};

export function phaseLabel(phase: CyclePhase): string {
	return phaseLabels[phase];
}

export function cycleName(cycle: Cycle): string {
	return cycle.name;
}

export function cyclePath(workspace: string, cycle: Cycle): string {
	return `/${workspace}/cycles/${cycle.teamKey.toLowerCase()}/${cycle.number}`;
}

export function teamCyclesPath(workspace: string, teamKey: string): string {
	return `/${workspace}/cycles/${teamKey.toLowerCase()}`;
}

export type CycleListing =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| { kind: "disabled"; teamKey: string }
	| { kind: "ready"; teamKey: string; teamName: string; cycles: Cycle[] };

export type CycleDetail =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| { kind: "ready"; cycle: Cycle; scope: CycleScope; nextNumber: number | null };

export type CadenceSetting =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "disabled" }
	| { kind: "enabled"; cadence: CycleCadence };

export type CycleFailure =
	| { kind: "closed" }
	| { kind: "overlaps" }
	| { kind: "team_mismatch" }
	| { kind: "rollover_required" }
	| { kind: "not_ended" }
	| { kind: "no_next_cycle" }
	| { kind: "invalid"; fields: string[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

const failureMessages: Record<CycleFailure["kind"], string> = {
	closed: "This cycle is closed. What happened in it can no longer be changed.",
	overlaps: "Those dates run into another cycle on this team.",
	team_mismatch: "That cycle belongs to a different team.",
	rollover_required: "Decide where the unfinished issues go before closing this cycle.",
	not_ended: "This cycle has not reached its end date yet.",
	no_next_cycle: "There is no later cycle to move these issues into.",
	invalid: "Check the highlighted fields and try again.",
	forbidden: "You do not have permission to change how this team runs cycles.",
	unavailable: "Something went wrong. Wait a moment and try again.",
};

export function cycleFailureMessage(failure: CycleFailure): string {
	return failureMessages[failure.kind];
}

const conflictKinds: Record<string, CycleFailure["kind"]> = {
	cycle_closed: "closed",
	cycle_overlaps: "overlaps",
	cycle_team_mismatch: "team_mismatch",
	cycle_rollover_required: "rollover_required",
	cycle_not_ended: "not_ended",
	cycle_no_next_cycle: "no_next_cycle",
};

export function readCycleFailure(problem: unknown): CycleFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	if ("code" in problem && typeof problem.code === "string") {
		const kind = conflictKinds[problem.code];
		if (kind) return { kind } as CycleFailure;
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

export function openIssues(issues: Issue[]): Issue[] {
	return issues.filter(
		(issue) => issue.state.category === "not_started" || issue.state.category === "active"
	);
}

export function scopeChangesOf(
	scope: CycleScope,
	kind: CycleScopeChange["change"]
): CycleScopeChange[] {
	return scope.changes.filter((change) => change.change === kind);
}
