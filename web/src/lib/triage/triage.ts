import type { components } from "$lib/api/dashboard.gen";
import type { Issue } from "$lib/issues/issues";
import type { Team } from "$lib/team/teams";

export type TriageQueue = components["schemas"]["TriageQueue"];
export type TriageSettings = components["schemas"]["TriageSettings"];
export type TriageSource = components["schemas"]["TriageSource"];
export type TriageState = components["schemas"]["TriageState"];

export type TriageListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; groups: TriageGroup[]; nextCursor?: string }
	| { kind: "unavailable" };

export type TriageGroup = { team: Team | null; issues: Issue[]; waiting: number };

export type TriageSetting =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "off" }
	| { kind: "on"; settings: TriageSettings };

export type TriageFailure =
	| { kind: "not_waiting" }
	| { kind: "relation_exists" }
	| { kind: "no_abandoned_state" }
	| { kind: "stale" }
	| { kind: "label_loss" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type TriageDeclineReason = components["schemas"]["TriageDeclineReason"];

export const sourceLabels: Record<TriageSource, string> = {
	user: "A person",
	token: "An integration",
	agent: "An agent",
};

export const sourceTabs: { value: "all" | TriageSource; label: string }[] = [
	{ value: "all", label: "All" },
	{ value: "user", label: "People" },
	{ value: "token", label: "Integrations" },
	{ value: "agent", label: "Agents" },
];

export const declineReasons: { value: TriageDeclineReason; label: string }[] = [
	{ value: "not_reproducible", label: "Not reproducible" },
	{ value: "working_as_intended", label: "Working as intended" },
	{ value: "out_of_scope", label: "Out of scope" },
	{ value: "no_response", label: "No response from the reporter" },
];

export const declineReasonLabels: Record<TriageDeclineReason, string> = Object.fromEntries(
	declineReasons.map((reason) => [reason.value, reason.label])
) as Record<TriageDeclineReason, string>;

export function queued(listing: TriageListing): Issue[] {
	if (listing.kind !== "ready") return [];

	return listing.groups
		.flatMap((group) => group.issues)
		.sort((first, second) => first.createdAt.localeCompare(second.createdAt));
}

const failureMessages: Record<TriageFailure["kind"], string> = {
	not_waiting: "Someone else already decided about this one.",
	relation_exists: "These two issues are already linked to each other in another way.",
	no_abandoned_state: "This team has no state to close issues into. Add one first.",
	stale: "Someone changed this issue while you were looking at it.",
	label_loss: "Some of its labels belong to this team and cannot travel with it.",
	forbidden: "You cannot decide about issues on this team.",
	unavailable: "Nothing changed. Wait a moment and try again.",
};

export function triageFailureMessage(failure: TriageFailure): string {
	return failureMessages[failure.kind];
}

const conflictKinds: Record<string, TriageFailure["kind"]> = {
	issue_not_waiting: "not_waiting",
	issue_relation_exists: "relation_exists",
	issue_destination_incapable: "no_abandoned_state",
	issue_stale: "stale",
	issue_labels_out_of_scope: "label_loss",
};

export function readTriageFailure(problem: unknown): TriageFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	if ("code" in problem && typeof problem.code === "string") {
		const kind = conflictKinds[problem.code];
		if (kind) return { kind } as TriageFailure;
	}

	if ("status" in problem && problem.status === 403) return { kind: "forbidden" };

	return { kind: "unavailable" };
}

export function listingFor(
	queue: TriageQueue | undefined,
	teams: Team[]
): TriageListing {
	if (!queue) return { kind: "unavailable" };
	if (queue.issues.length === 0) return { kind: "empty" };

	const waiting = new Map(queue.teams.map((tally) => [tally.key, tally.issues]));
	const groups: TriageGroup[] = [];

	for (const issue of queue.issues) {
		let group = groups.find((candidate) => candidate.team?.id === issue.teamId);

		if (!group) {
			group = {
				team: teams.find((team) => team.id === issue.teamId) ?? null,
				issues: [],
				waiting: waiting.get(issue.teamId) ?? 0,
			};

			groups.push(group);
		}

		group.issues.push(issue);
	}

	return { kind: "ready", groups, nextCursor: queue.nextCursor };
}

export function waitingTotal(queue: TriageQueue | undefined): number {
	if (!queue) return 0;

	return queue.teams.reduce((sum, tally) => sum + tally.issues, 0);
}

export function settingFor(
	settings: TriageSettings | undefined,
	status: number
): TriageSetting {
	if (settings) return { kind: "on", settings };
	if (status === 404) return { kind: "off" };

	return { kind: "unavailable" };
}
