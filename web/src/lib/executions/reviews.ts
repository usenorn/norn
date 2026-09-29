import type { ExecutionSummary } from "./executions";

export type ReviewQueue =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "ready"; runs: ExecutionSummary[] };

export const noReviewsLine =
	"Nothing is waiting. A run appears here when a coding agent has a plan for somebody to approve, or changes for somebody to review.";

export function plansWaiting(runs: ExecutionSummary[]): ExecutionSummary[] {
	return runs.filter((run) => run.execution.state === "awaiting_plan_approval");
}

export function changesWaiting(runs: ExecutionSummary[]): ExecutionSummary[] {
	return runs.filter((run) => run.execution.state === "awaiting_review");
}

export function waitingCount(runs: ExecutionSummary[]): string {
	return runs.length === 1 ? "1 run is waiting" : `${runs.length} runs are waiting`;
}
