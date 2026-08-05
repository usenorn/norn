import type { components, operations } from "$lib/api/dashboard.gen";

export type Agent = components["schemas"]["Agent"];
export type WorkspaceAgent = components["schemas"]["WorkspaceAgent"];
export type AgentSettings = components["schemas"]["AgentSettings"];
export type AgentProposal = components["schemas"]["AgentProposal"];
export type AgentAction = components["schemas"]["AgentAction"];
export type APIScope = components["schemas"]["APIScope"];

type RegisterResponses = operations["registerWorkspaceAgent"]["responses"];

type CodedRegisterProblem = RegisterResponses[409]["content"]["application/problem+json"];

export type RegisterProblem =
	RegisterResponses[403 | 409 | 422 | 500]["content"]["application/problem+json"];

export type AgentListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; agents: WorkspaceAgent[] }
	| { kind: "registered"; agents: WorkspaceAgent[]; agent: Agent; value: string }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type AgentFailure =
	| { kind: "name_taken" }
	| { kind: "owner_invalid" }
	| { kind: "disabled" }
	| { kind: "scope_exceeds" }
	| { kind: "scope_invalid" }
	| { kind: "grant_invalid" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type ProposalQueue =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; proposals: AgentProposal[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const actionLabels: Record<AgentAction, string> = {
	comment: "Leave a comment",
	state_change: "Change the state",
	issue_edit: "Edit the issue",
};

export const holdLabels: { key: keyof AgentSettings; title: string; detail: string }[] = [
	{
		key: "holdComments",
		title: "Comments",
		detail: "An agent's comment waits until somebody agrees it should be said.",
	},
	{
		key: "holdStateChanges",
		title: "State changes",
		detail: "An agent cannot move work through this team's board on its own.",
	},
	{
		key: "holdIssueEdits",
		title: "Issue edits",
		detail: "Titles, priorities and assignees an agent changes wait for a person.",
	},
];

export const agentScopeGroups: { title: string; scopes: APIScope[] }[] = [
	{ title: "Issues", scopes: ["issue:read", "issue:manage"] },
	{ title: "Projects", scopes: ["project:read", "project:manage"] },
	{ title: "Labels", scopes: ["label:read", "label:manage"] },
	{ title: "Teams", scopes: ["team:read", "team:manage"] },
	{ title: "Members", scopes: ["membership:read"] },
	{ title: "Comments", scopes: ["comment:read", "comment:manage"] },
	{ title: "Its own inbox", scopes: ["notification:read", "notification:manage"] },
];

export const agentScopeLabels: Record<string, string> = {
	"issue:read": "Read issues",
	"issue:manage": "Raise and change issues",
	"project:read": "Read projects",
	"project:manage": "Create and change projects",
	"label:read": "Read labels",
	"label:manage": "Create and change labels",
	"team:read": "Read teams",
	"team:manage": "Create and change teams",
	"membership:read": "Read members",
	"comment:read": "Read comments",
	"comment:manage": "Leave comments",
	"notification:read": "Read what it has been assigned or mentioned in",
	"notification:manage": "Mark its own inbox as read",
};

function coded(problem: RegisterProblem): problem is CodedRegisterProblem {
	return "code" in problem;
}

export function registerFailure(problem: RegisterProblem): AgentFailure {
	if (!coded(problem)) return problem.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

	switch (problem.code) {
		case "agent_name_taken":
			return { kind: "name_taken" };
		case "agent_owner_invalid":
			return { kind: "owner_invalid" };
		case "agent_disabled":
			return { kind: "disabled" };
		case "agent_proposal_settled":
			return { kind: "unavailable" };
		default:
			return { kind: "unavailable" };
	}
}

export function failureMessage(failure: AgentFailure): string {
	switch (failure.kind) {
		case "name_taken":
			return "An agent in this workspace already has that name.";
		case "owner_invalid":
			return "That person is not a member of this workspace, so they cannot own an agent here.";
		case "disabled":
			return "That agent has been disabled.";
		case "scope_exceeds":
			return "An agent cannot do more than the person it acts for. Choose fewer permissions, or a different owner.";
		case "scope_invalid":
			return "One of those permissions is not recognised.";
		case "grant_invalid":
			return "One of those teams is not one you can reach.";
		case "forbidden":
			return "You may not manage agents in this workspace.";
		case "unavailable":
			return "We could not reach the server. Try again in a moment.";
	}
}

export function proposalSummary(proposal: AgentProposal): string {
	switch (proposal.action) {
		case "comment":
			return proposal.body ? `“${proposal.body}”` : "a comment";
		case "state_change":
			return "a new state";
		case "issue_edit":
			return proposal.title ? `the title “${proposal.title}”` : "an edit";
	}
}

export function agentsPath(workspace: string): string {
	return `/${workspace}/settings/agents`;
}

export function agentPath(workspace: string, agentId: string): string {
	return `/${workspace}/settings/agents/${agentId}`;
}

export function approvalsPath(workspace: string): string {
	return `/${workspace}/agents/approvals`;
}
