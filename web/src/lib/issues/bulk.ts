import type { components } from "$lib/api/dashboard.gen";

export type BulkOutcome = components["schemas"]["BulkOutcome"];
export type BulkActionStatus = components["schemas"]["BulkActionStatus"];
export type BulkActionResult = components["schemas"]["BulkActionResult"];
export type BulkActionOutcome = components["schemas"]["BulkActionOutcome"];

const outcomeCopy: Record<BulkOutcome, string> = {
	applied: "Changed",
	unchanged: "Already as asked",
	forbidden: "You cannot change this one",
	not_found: "No longer there",
	conflict: "Someone edited it first",
	invalid: "The change does not fit this issue",
};

export function outcomeLabel(outcome: BulkOutcome): string {
	return outcomeCopy[outcome] ?? "Did not apply";
}

export function applied(outcome: BulkOutcome): boolean {
	return outcome === "applied" || outcome === "unchanged";
}

export function failures(result: BulkActionResult): BulkActionOutcome[] {
	return result.outcomes.filter((outcome) => !applied(outcome.outcome));
}

export function settled(status: BulkActionStatus): boolean {
	return status === "complete" || status === "failed";
}

export function summary(result: BulkActionResult): string {
	const changed = result.outcomes.filter((outcome) => applied(outcome.outcome)).length;
	const failed = failures(result).length;

	if (!settled(result.status)) {
		return result.expected == null
			? `${result.processed} changed so far`
			: `${result.processed} of ${result.expected} changed`;
	}

	if (failed === 0) return `${changed} ${changed === 1 ? "issue" : "issues"} changed`;

	return `${changed} changed, ${failed} did not`;
}

export function rangeBetween(ids: string[], from: string, to: string): string[] {
	const start = ids.indexOf(from);
	const end = ids.indexOf(to);

	if (start < 0 || end < 0) return [];

	return ids.slice(Math.min(start, end), Math.max(start, end) + 1);
}
