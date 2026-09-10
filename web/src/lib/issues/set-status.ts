import { api } from "$lib/api";
import { attempt, unknownLine } from "$lib/api/attempt";
import type { WorkflowState } from "$lib/team/states";
import type { Issue } from "./issues";

export type StatusOutcome =
	| { kind: "changed"; state: WorkflowState }
	| { kind: "unchanged" }
	| { kind: "stale" }
	| { kind: "uncertain" }
	| { kind: "unavailable" };

export function nthState(
	states: WorkflowState[],
	teamId: string,
	nth: number
): WorkflowState | undefined {
	return states
		.filter((state) => state.teamId === teamId)
		.sort((a, b) => a.position - b.position || a.name.localeCompare(b.name))[nth];
}

export function statusIndexOf(binding: string): number {
	return Number(binding) - 1;
}

export async function setStatus(
	workspaceId: string,
	issue: Issue,
	state: WorkflowState
): Promise<StatusOutcome> {
	if (issue.state.id === state.id) return { kind: "unchanged" };

	const outcome = await attempt({
		run: () =>
			api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId, issueId: issue.id } },
				body: { stateId: state.id, expectedVersion: issue.version },
			}),
	});

	if (outcome.kind === "done") return { kind: "changed", state };
	if (outcome.kind === "unknown") return { kind: "uncertain" };

	return outcome.status === 409 ? { kind: "stale" } : { kind: "unavailable" };
}

export function statusMessage(outcome: StatusOutcome, reference: string): string {
	switch (outcome.kind) {
		case "changed":
			return `Moved ${reference} to ${outcome.state.name}`;
		case "stale":
			return `${reference} changed while you were looking. Nothing moved — reload and try again.`;
		case "uncertain":
			return unknownLine;
		case "unavailable":
			return `${reference} did not move. Nothing changed — try again in a moment.`;
		case "unchanged":
			return "";
	}
}
