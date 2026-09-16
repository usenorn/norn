import { api } from "$lib/api";
import { attempt } from "$lib/api/attempt";
import { expectedVersion, remember } from "$lib/issues/versions";
import type { Issue } from "$lib/issues/issues";

export type LabelsOutcome =
	| { kind: "changed"; issue: Issue }
	| { kind: "superseded" }
	| { kind: "refused"; problem: unknown; status: number }
	| { kind: "uncertain" };

export type LabelChange = {
	workspaceId: string;
	issue: Issue;
	labelIds: string[];
	optimistic?: () => void;
	reconcile?: () => void;
};

type Waiting = { change: LabelChange; settle: (outcome: LabelsOutcome) => void };

const inFlight = new Map<string, Waiting | null>();

export function setIssueLabels(change: LabelChange): Promise<LabelsOutcome> {
	change.optimistic?.();

	const issueId = change.issue.id;

	if (!inFlight.has(issueId)) {
		inFlight.set(issueId, null);

		return send(change);
	}

	return new Promise((settle) => {
		inFlight.get(issueId)?.settle({ kind: "superseded" });
		inFlight.set(issueId, { change, settle });
	});
}

async function send(change: LabelChange): Promise<LabelsOutcome> {
	const outcome = await attempt({
		run: () =>
			api.PUT("/workspaces/{workspaceId}/issues/{issueId}/labels", {
				params: { path: { workspaceId: change.workspaceId, issueId: change.issue.id } },
				body: { expectedVersion: expectedVersion(change.issue), labelIds: change.labelIds },
			}),
		reconcile: change.reconcile,
	});

	if (outcome.kind === "done") remember(outcome.value);

	const waiting = inFlight.get(change.issue.id);

	if (waiting) {
		inFlight.set(change.issue.id, null);
		void send(waiting.change).then(waiting.settle);
	} else {
		inFlight.delete(change.issue.id);
	}

	switch (outcome.kind) {
		case "done":
			return { kind: "changed", issue: outcome.value };
		case "refused":
			return { kind: "refused", problem: outcome.problem, status: outcome.status };
		default:
			return { kind: "uncertain" };
	}
}
