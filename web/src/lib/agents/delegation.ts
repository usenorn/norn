import type { components, operations } from "$lib/api/dashboard.gen";
import { isSettled, type Execution } from "$lib/executions/executions";

export type IssueDelegation = components["schemas"]["IssueDelegation"];
export type Member = components["schemas"]["Membership"];

export type DelegationPanel =
	| { kind: "loading" }
	| { kind: "none" }
	| { kind: "held"; delegation: IssueDelegation; running: boolean }
	| { kind: "unavailable" };

type DelegateResponses = operations["delegateWorkspaceIssue"]["responses"];

type CodedDelegateProblem = DelegateResponses[409]["content"]["application/problem+json"];

export type DelegateProblem =
	DelegateResponses[403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export type DelegationFailure =
	| { kind: "held" }
	| { kind: "agent_unusable" }
	| { kind: "unassigned" }
	| { kind: "gone" }
	| { kind: "invalid" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

function coded(problem: DelegateProblem): problem is CodedDelegateProblem {
	return "code" in problem && typeof problem.code === "string";
}

export function readDelegationFailure(error: unknown): DelegationFailure {
	if (!error || typeof error !== "object") return { kind: "unavailable" };

	const problem = error as DelegateProblem & { errors?: unknown[]; status?: number };

	if (coded(problem)) {
		if (problem.code === "issue_delegation_held") return { kind: "held" };
		if (problem.code === "issue_delegation_agent_unusable") return { kind: "agent_unusable" };
		if (problem.code === "issue_delegation_unassigned") return { kind: "unassigned" };
	}

	if (problem.errors) return { kind: "invalid" };
	if (problem.status === 404) return { kind: "gone" };
	if (problem.status === 403) return { kind: "forbidden" };

	return { kind: "unavailable" };
}

export function delegationFailureMessage(failure: DelegationFailure): string {
	switch (failure.kind) {
		case "held":
			return "An agent is working on this right now. Take it back before handing it over again.";
		case "agent_unusable":
			return "That agent is disabled, so it cannot take work.";
		case "unassigned":
			return "Somebody cleared the assignee while you were deciding. Assign it again before handing it over.";
		case "gone":
			return "Nobody is holding this issue.";
		case "invalid":
			return "That brief is longer than Norn will carry. Shorten it and try again.";
		case "forbidden":
			return "You may not hand this issue to an agent.";
		default:
			return "Something went wrong and nothing changed. Wait a moment and try again.";
	}
}

export function currentDelegation(
	delegations: IssueDelegation[],
	runs: Execution[]
): DelegationPanel {
	const held = delegations.find((delegation) => !delegation.recalledAt);

	if (!held) return { kind: "none" };

	return {
		kind: "held",
		delegation: held,
		running: runs.some((run) => run.delegationId === held.id && !isSettled(run.state)),
	};
}

export function agentMembers(members: Member[]): Member[] {
	return members.filter((member) => member.kind === "agent" && !member.deactivatedAt);
}
