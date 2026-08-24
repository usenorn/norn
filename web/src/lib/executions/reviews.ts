import type { ExecutionSummary } from "./executions";

export type ReviewQueue =
	| { kind: "loading" }
	| { kind: "unavailable" }
	| { kind: "ready"; runs: ExecutionSummary[] };

export const noReviewsLine =
	"Nothing is waiting. A run appears here when a coding agent has finished and somebody has to say whether the work is good.";

export function waitingCount(runs: ExecutionSummary[]): string {
	return runs.length === 1 ? "1 run is waiting" : `${runs.length} runs are waiting`;
}
