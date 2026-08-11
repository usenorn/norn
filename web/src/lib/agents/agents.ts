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

export type AgentHold = components["schemas"]["AgentHold"];

export const actionLabels: Record<AgentAction, string> = {
	comment: "Leave a comment",
	state_change: "Change the state",
	issue_edit: "Edit the issue",
	check_set: "Add to what done means",
};

export const holdOptionLabels: Record<AgentHold, string> = {
	never: "Never hold",
	unless_proven: "Hold unless proven",
	always: "Always hold",
};

export const holdOptionHints: Record<AgentHold, string> = {
	never: "An agent with the right permissions does this straight away.",
	always: "Every one of these waits for a person, however well evidenced.",
	unless_proven:
		"A finish whose checks all pass goes through. Everything else waits. An issue carrying no approved checks has nothing to prove, so it is not held.",
};

export const holdLabels: {
	key: keyof AgentSettings;
	title: string;
	detail: string;
	options: AgentHold[];
}[] = [
	{
		key: "holdComments",
		title: "Comments",
		detail: "An agent's comment waits until somebody agrees it should be said.",
		options: ["never", "always"],
	},
	{
		key: "holdStateChanges",
		title: "State changes",
		detail: "An agent cannot move work through this team's board on its own.",
		options: ["never", "unless_proven", "always"],
	},
	{
		key: "holdIssueEdits",
		title: "Issue edits",
		detail: "Titles, priorities and assignees an agent changes wait for a person.",
		options: ["never", "always"],
	},
];

export const agentScopeGroups: { title: string; scopes: APIScope[] }[] = [
	{ title: "Issues", scopes: ["issue:read", "issue:manage"] },
	{ title: "Cycles", scopes: ["cycle:read"] },
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
	"cycle:read": "Read cycles",
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
		case "check_set":
			return proposal.checkIds?.length === 1
				? "one criterion for what done means here"
				: `${proposal.checkIds?.length ?? 0} criteria for what done means here`;
	}
}

export function mcpServerUrl(origin: string): string {
	return `${origin}/mcp`;
}

export function mcpServerName(agentName: string): string {
	const slug = agentName
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "-")
		.replace(/^-+|-+$/g, "");

	return slug || "norn";
}

export function mcpAddCommand(agentName: string, origin: string, value: string): string {
	return (
		`claude mcp add --transport http ${mcpServerName(agentName)} ${mcpServerUrl(origin)} ` +
		`--header "Authorization: Bearer ${value}"`
	);
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
