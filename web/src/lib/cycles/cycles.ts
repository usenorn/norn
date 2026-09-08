import type { components } from "$lib/api/dashboard.gen";
import type { Issue } from "$lib/issues/issues";
import { categoryLabels, type StateCategory } from "$lib/team/states";

export type Cycle = components["schemas"]["Cycle"];
export type CyclePhase = components["schemas"]["CyclePhase"];
export type CycleRollover = components["schemas"]["CycleRollover"];
export type CycleCadence = components["schemas"]["CycleCadence"];
export type CycleScopeChange = components["schemas"]["CycleScopeChange"];
export type CycleScope = components["schemas"]["CycleScope"];
export type TeamCycle = components["schemas"]["TeamCycle"];
export type CycleReport = components["schemas"]["CycleReport"];
export type CycleBurndown = components["schemas"]["CycleBurndown"];
export type CycleBurndownPoint = components["schemas"]["CycleBurndownPoint"];
export type CycleResult = components["schemas"]["CycleResult"];

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
	| { kind: "disabled"; teamKey: string; teamName: string }
	| { kind: "ready"; teamKey: string; teamName: string; cycles: Cycle[] };

export type CycleDetail =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			cycle: Cycle;
			scope: CycleScope;
			report: CycleReport;
			others: Cycle[];
			previous: CycleReport | undefined;
			nextNumber: number | null;
	  };

export type CycleStanding =
	| { kind: "upcoming" }
	| { kind: "running"; day: number; of: number }
	| { kind: "ending"; daysLeft: number }
	| { kind: "ended" }
	| { kind: "closed" };

export function cycleStanding(cycle: Cycle, today: string): CycleStanding {
	if (cycle.phase === "closed") return { kind: "closed" };
	if (today < cycle.startsOn) return { kind: "upcoming" };
	if (today > cycle.endsOn) return { kind: "ended" };

	const of = calendarSpan(cycle.startsOn, cycle.endsOn);
	const day = calendarSpan(cycle.startsOn, today);
	const daysLeft = of - day;

	return daysLeft <= 2 ? { kind: "ending", daysLeft } : { kind: "running", day, of };
}

export function calendarSpan(from: string, to: string): number {
	const start = Date.parse(`${from}T00:00:00Z`);
	const end = Date.parse(`${to}T00:00:00Z`);

	if (Number.isNaN(start) || Number.isNaN(end)) return 0;

	return Math.round((end - start) / 86_400_000) + 1;
}

export const standingLabels: Record<CycleStanding["kind"], string> = {
	upcoming: "Not started",
	running: "Active",
	ending: "Ending soon",
	ended: "Ended",
	closed: "Closed",
};

export function plannedDays(cycle: Cycle): number {
	return calendarSpan(cycle.startsOn, cycle.endsOn);
}

export function burndownKnown(point: CycleBurndownPoint): boolean {
	return !point.unrecorded && point.unknown === 0;
}

export function burndownStart(burndown: CycleBurndown): number | null {
	const first = burndown.points[0];

	return first && burndownKnown(first) ? first.scope : null;
}

export function burndownIdeal(cycle: Cycle, burndown: CycleBurndown): number[] {
	const days = plannedDays(cycle);
	const start = burndownStart(burndown);

	if (days === 0 || start === null) return [];
	if (days === 1) return [start];

	return Array.from({ length: days }, (_, index) => start - (start * index) / (days - 1));
}

export type BurndownRun = { from: number; points: CycleBurndownPoint[] };

export function burndownRuns(burndown: CycleBurndown): BurndownRun[] {
	const runs: BurndownRun[] = [];

	burndown.points.forEach((point, index) => {
		if (!burndownKnown(point)) return;

		const last = runs[runs.length - 1];

		if (last && last.from + last.points.length === index) {
			last.points.push(point);

			return;
		}

		runs.push({ from: index, points: [point] });
	});

	return runs;
}

export function burndownCeiling(burndown: CycleBurndown): number {
	return burndown.points.reduce((highest, point) => Math.max(highest, point.scope), 0);
}

export function unrecordedDays(burndown: CycleBurndown): number {
	return burndown.points.filter((point) => point.unrecorded).length;
}

export function unknownDays(burndown: CycleBurndown): number {
	return burndown.points.filter((point) => !point.unrecorded && point.unknown > 0).length;
}

export function scopeAddedWithin(scope: CycleScope, cycle: Cycle): number {
	const seen = new Set<string>();

	for (const change of scope.changes) {
		if (change.change !== "added") continue;
		if (cycle.closedAt && change.changedAt > cycle.closedAt) continue;

		seen.add(change.issueId);
	}

	return seen.size;
}

export type CycleResults =
	| { kind: "live"; issues: Issue[] }
	| { kind: "final"; issues: Issue[]; results: CycleResult[] }
	| { kind: "unrecorded"; issues: Issue[] };

export function cycleResults(report: CycleReport): CycleResults {
	if (report.phase !== "closed") return { kind: "live", issues: report.issues };
	if (!report.frozen) return { kind: "unrecorded", issues: report.issues };

	return { kind: "final", issues: report.issues, results: report.results };
}

export function resultIssues(results: CycleResults): Issue[] {
	return results.issues;
}

function frozenResult(results: CycleResults, issue: Issue): CycleResult | undefined {
	return results.kind === "final"
		? results.results.find((result) => result.issueId === issue.id)
		: undefined;
}

export function categoryOf(results: CycleResults, issue: Issue): StateCategory {
	return frozenResult(results, issue)?.category ?? issue.state.category;
}

export function stateNameOf(results: CycleResults, issue: Issue): string {
	return frozenResult(results, issue)?.stateName ?? issue.state.name;
}

export function issueAsRecorded(results: CycleResults, issue: Issue): Issue {
	const result = frozenResult(results, issue);

	if (!result) return issue;

	return {
		...issue,
		state: { ...issue.state, name: result.stateName, category: result.category },
	};
}

export function decisionOf(results: CycleResults, issue: Issue): CycleRollover | undefined {
	return frozenResult(results, issue)?.decision;
}

export type CycleCounts = {
	scope: number;
	done: number;
	inFlight: number;
	notStarted: number;
	dropped: number;
};

export function cycleCounts(results: CycleResults): CycleCounts {
	const counts: CycleCounts = { scope: 0, done: 0, inFlight: 0, notStarted: 0, dropped: 0 };

	for (const issue of resultIssues(results)) {
		counts.scope += 1;

		switch (categoryOf(results, issue)) {
			case "complete":
				counts.done += 1;
				break;
			case "active":
				counts.inFlight += 1;
				break;
			case "not_started":
				counts.notStarted += 1;
				break;
			default:
				counts.dropped += 1;
		}
	}

	return counts;
}

const categoryOrder: StateCategory[] = ["active", "not_started", "complete", "abandoned"];

export type CycleGroup = { category: StateCategory; label: string; issues: Issue[] };

export function cycleGroups(results: CycleResults): CycleGroup[] {
	return categoryOrder
		.map((category) => ({
			category,
			label: categoryLabels[category],
			issues: resultIssues(results).filter((issue) => categoryOf(results, issue) === category),
		}))
		.filter((group) => group.issues.length > 0);
}

export type CycleRisk = { issue: Issue; reason: string };

export function cycleRisks(results: CycleResults): CycleRisk[] {
	if (results.kind !== "live") return [];

	const risks: CycleRisk[] = [];

	for (const issue of results.issues) {
		const category = categoryOf(results, issue);

		if (category !== "active" && category !== "not_started") continue;

		if (issue.blocked) {
			risks.push({ issue, reason: "Blocked by another issue" });

			continue;
		}

		if (!issue.assigneeAccountId) risks.push({ issue, reason: "Nobody is assigned" });
	}

	return risks;
}

export function unfinishedIssues(results: CycleResults): Issue[] {
	return resultIssues(results).filter((issue) => {
		const category = categoryOf(results, issue);

		return category === "active" || category === "not_started";
	});
}

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
	| { kind: "stale" }
	| { kind: "owner_not_on_team" }
	| { kind: "uncertain" }
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
	stale: "The unfinished work changed while this form was open. Review it again before closing.",
	uncertain:
		"We could not read this cycle back, so we cannot tell whether it closed. Reload the page " +
		"before trying again.",
	owner_not_on_team: "Only somebody on this team can own its cycles.",
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
	cycle_stale: "stale",
	cycle_owner_not_on_team: "owner_not_on_team",
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

export function rolloverLabels(next: string): Record<CycleRollover, string> {
	return {
		next: `Move them to ${next}`,
		backlog: "Return them to the backlog",
		keep: "Keep them in this cycle",
	};
}

export function destinationLabels(next: string): Record<CycleRollover, string> {
	return { next, backlog: "Backlog", keep: "Keep here" };
}
