import type { components, operations } from "$lib/api/dashboard.gen";

export type IssueCheck = components["schemas"]["IssueCheck"];
export type IssueCheckList = components["schemas"]["IssueCheckList"];
export type IssueCheckSummary = components["schemas"]["IssueCheckSummary"];
export type CheckEvidence = components["schemas"]["CheckEvidence"];
export type CheckState = components["schemas"]["CheckState"];
export type CheckMethod = components["schemas"]["CheckMethod"];
export type CheckAwaiting = components["schemas"]["CheckAwaiting"];
export type EvidenceVerdict = components["schemas"]["EvidenceVerdict"];
export type EvidenceChannel = components["schemas"]["EvidenceChannel"];

export type ChecksPanel =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; checks: IssueCheck[]; summary: IssueCheckSummary }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type EvidencePanel =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; evidence: CheckEvidence[] }
	| { kind: "unavailable" };

type CheckResponses = operations["addWorkspaceIssueChecks"]["responses"];

type CodedCheckProblem = CheckResponses[409]["content"]["application/problem+json"];

export type CheckProblem =
	CheckResponses[403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export type CheckFailure =
	| { kind: "settled" }
	| { kind: "decided" }
	| { kind: "declined" }
	| { kind: "limit_reached" }
	| { kind: "not_personal" }
	| { kind: "evidence_empty" }
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

function coded(problem: CheckProblem): problem is CodedCheckProblem {
	return "code" in problem;
}

export function readCheckFailure(problem: CheckProblem): CheckFailure {
	if (!coded(problem)) {
		if (problem.status === 403) return { kind: "forbidden" };
		if (problem.status === 404) return { kind: "gone" };

		return { kind: "unavailable" };
	}

	switch (problem.code) {
		case "check_settled":
			return { kind: "settled" };
		case "check_decided":
			return { kind: "decided" };
		case "check_declined":
			return { kind: "declined" };
		case "check_limit_reached":
			return { kind: "limit_reached" };
		case "check_decision_not_personal":
		case "check_waiver_not_personal":
			return { kind: "not_personal" };
		case "evidence_empty":
			return { kind: "evidence_empty" };
		default:
			return { kind: "unavailable" };
	}
}

export function checkFailureMessage(failure: CheckFailure): string {
	switch (failure.kind) {
		case "settled":
			return "That criterion has already been waived or recorded as a gap.";
		case "decided":
			return "Somebody has already approved or declined that criterion.";
		case "declined":
			return "That criterion was declined, so it no longer takes evidence.";
		case "limit_reached":
			return "This issue already carries as many criteria as it may.";
		case "not_personal":
			return "Only a person can approve or waive a criterion. An agent cannot settle what it is graded against.";
		case "evidence_empty":
			return "Evidence needs the output it was drawn from, not a description of it.";
		case "gone":
			return "That criterion is no longer here. Reload the issue.";
		case "forbidden":
			return "You may not change what done means on this issue.";
		case "unavailable":
			return "We could not reach the server. Try again in a moment.";
	}
}

export const checkStateLabels: Record<CheckState, string> = {
	unproven: "Unproven",
	proven: "Proven",
	failed: "Failed",
	waived: "Waived",
	gap: "Gap",
};

export const checkStateTones: Record<CheckState, string> = {
	unproven: "text-muted-foreground",
	proven: "text-success",
	failed: "text-destructive",
	waived: "text-muted-foreground",
	gap: "text-warning",
};

export const methodLabels: Record<CheckMethod, string> = {
	command: "Command",
	observation: "Observation",
	manual: "Manual",
	regression: "Regression",
};

export const methodHints: Record<CheckMethod, string> = {
	command: "Something runnable, proven by what it printed.",
	observation: "A signal somebody reads — a log, a dashboard, a response.",
	manual: "Somebody has to look. Only a person's word proves it.",
	regression: "Has to fail before the fix and pass after, so the test is shown to catch it.",
};

export const awaitingLabels: Record<CheckAwaiting, string> = {
	correction:
		"The newest result on this disproves it. Fix the work, then file a passing result.",
	fresh_proof: "The proof of this timed out, so it no longer counts. Run it again.",
	attestation:
		"A person has to attest this. A passing result an agent files does not prove it, however carefully it looked.",
	prior_failure:
		"Still waiting for a failing result filed before the passing one, so the test is shown to catch what it is for.",
	positive_result:
		"Nothing filed here is a positive result. Absence of a failure never proves a criterion.",
	evidence: "Nothing has been filed against this yet.",
};

export const verdictLabels: Record<EvidenceVerdict, string> = {
	passed: "Passed",
	failed: "Failed",
	absent_negative: "Nothing bad seen",
	inconclusive: "Inconclusive",
};

export const verdictTones: Record<EvidenceVerdict, string> = {
	passed: "text-success",
	failed: "text-destructive",
	absent_negative: "text-muted-foreground",
	inconclusive: "text-muted-foreground",
};

export const channelLabels: Record<EvidenceChannel, string> = {
	command: "Command",
	http: "HTTP",
	log: "Log",
	screenshot: "Screenshot",
	database: "Database",
	human: "A person looked",
};

export function summaryLine(summary: IssueCheckSummary): string {
	if (summary.total === 0) return "Nothing here says when this is finished.";

	const settled = summary.proven + summary.waived + summary.gaps;

	return `${settled} of ${summary.total} settled`;
}

export function blockingLine(summary: IssueCheckSummary): string {
	if (summary.blocking === 0) return "";

	return summary.blocking === 1
		? "One criterion is not proven."
		: `${summary.blocking} criteria are not proven.`;
}

export function unprovenNames(checks: IssueCheck[]): IssueCheck[] {
	return checks.filter((check) => check.blocking);
}

export function proposed(checks: IssueCheck[]): IssueCheck[] {
	return checks.filter((check) => check.approval === "pending");
}

export function onlyTimeLimitHolds(evidence: CheckEvidence): boolean {
	return evidence.verdict === "passed" && !evidence.codeLinkId;
}

export function evidenceAge(check: IssueCheck): string {
	const seconds = check.timeLimitSeconds ?? 0;
	if (seconds <= 0) return "";

	const days = Math.round(seconds / 86400);
	if (days >= 1) return `${days} ${days === 1 ? "day" : "days"}`;

	const hours = Math.max(1, Math.round(seconds / 3600));

	return `${hours} ${hours === 1 ? "hour" : "hours"}`;
}
