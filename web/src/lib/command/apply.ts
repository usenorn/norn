import { api } from "$lib/api";
import { attempt, unknownLine } from "$lib/api/attempt";
import type { components } from "$lib/api/dashboard.gen";
import { failures, outcomeLabel, settled } from "$lib/issues/bulk";
import { issueFailureMessage, readIssueFailure } from "$lib/issues/issues";

export type BulkChange = components["schemas"]["BulkChange"];

export type ChangeOutcome =
	| { kind: "applied" }
	| { kind: "queued" }
	| { kind: "refused"; detail: string };

export async function applyChange(
	workspaceId: string,
	issueIds: string[],
	change: BulkChange
): Promise<ChangeOutcome> {
	const outcome = await attempt({
		run: () =>
			api.POST("/workspaces/{workspaceId}/issues/bulk", {
				params: { path: { workspaceId } },
				body: { change, issueIds },
			}),
	});

	if (outcome.kind === "unknown") return { kind: "refused", detail: unknownLine };

	if (outcome.kind === "refused") {
		return { kind: "refused", detail: issueFailureMessage(readIssueFailure(outcome.problem)) };
	}

	if (!settled(outcome.value.status)) return { kind: "queued" };

	const failed = failures(outcome.value);

	if (failed.length === 0) return { kind: "applied" };

	return {
		kind: "refused",
		detail: failed
			.map((entry) => `${entry.reference} · ${outcomeLabel(entry.outcome)}`)
			.join(". "),
	};
}
