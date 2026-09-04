import type { components, operations } from "$lib/api/dashboard.gen";

export type Agent = components["schemas"]["Agent"];
export type WorkspaceAgent = components["schemas"]["WorkspaceAgent"];
export type AgentSettings = components["schemas"]["AgentSettings"];
export type AgentProposal = components["schemas"]["AgentProposal"];
export type AgentAction = components["schemas"]["AgentAction"];
export type APIScope = components["schemas"]["APIScope"];
export type AgentIcon = components["schemas"]["AgentIcon"];

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
	| { kind: "authority_missing" }
	| { kind: "unavailable" };

export type AgentFailure =
	| { kind: "name_taken" }
	| { kind: "owner_invalid" }
	| { kind: "lifecycle_owner_invalid" }
	| { kind: "disabled" }
	| { kind: "active" }
	| { kind: "authority_missing" }
	| { kind: "scope_exceeds"; scopes: AgentScope[] }
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
export type AgentReasoning = components["schemas"]["AgentReasoning"];

export const actionLabels: Record<AgentAction, string> = {
	comment: "Leave a comment",
	state_change: "Change the state",
	issue_edit: "Edit the issue",
	issue_create: "Raise an issue",
};

export const agentIcons = [
	"bot",
	"inbox",
	"search",
	"terminal",
	"pencil",
	"git-pull-request",
	"shield-check",
	"scroll-text",
	"target",
	"sparkles",
] as const satisfies readonly AgentIcon[];

export const agentIconLabels: Record<AgentIcon, string> = {
	bot: "General",
	inbox: "Triage",
	search: "Research",
	terminal: "Engineering",
	pencil: "Writing",
	"git-pull-request": "Review",
	"shield-check": "Security",
	"scroll-text": "Documentation",
	target: "Planning",
	sparkles: "Creative",
};

export const agentScopes = [
	"issue:read",
	"issue:manage",
	"cycle:read",
	"project:read",
	"project:manage",
	"label:read",
	"label:manage",
	"team:read",
	"team:manage",
	"membership:read",
	"comment:read",
	"comment:manage",
	"notification:read",
	"notification:manage",
] as const satisfies readonly APIScope[];

export type AgentScope = (typeof agentScopes)[number];

export const holdOptionLabels: Record<AgentHold, string> = {
	never: "Never hold",
	always: "Always hold",
};

export const holdOptionHints: Record<AgentHold, string> = {
	never: "An agent with the right permissions does this straight away.",
	always: "Every one of these waits for a person.",
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
		options: ["never", "always"],
	},
	{
		key: "holdIssueEdits",
		title: "Issue edits",
		detail: "Titles, priorities and assignees an agent changes wait for a person.",
		options: ["never", "always"],
	},
	{
		key: "holdIssueCreation",
		title: "New issues",
		detail: "An issue an agent raises on this team waits before it reaches anyone's board.",
		options: ["never", "always"],
	},
];

export const agentScopeGroups: { title: string; scopes: AgentScope[] }[] = [
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

export function beyondGrant(scopes: AgentScope[], grantable: APIScope[] | null): AgentScope[] {
	if (!grantable) return [];

	return scopes.filter((scope) => !grantable.includes(scope));
}

function listed(names: string[]): string {
	if (names.length === 1) return `“${names[0]}”`;

	const quoted = names.map((name) => `“${name}”`);

	return `${quoted.slice(0, -1).join(", ")} and ${quoted[quoted.length - 1]}`;
}

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
		case "agent_active":
			return { kind: "active" };
		case "agent_authority_missing":
			return { kind: "authority_missing" };
		case "agent_proposal_settled":
			return { kind: "unavailable" };
		case "token_scope_invalid":
			return { kind: "scope_invalid" };
		case "token_scope_exceeds":
			return { kind: "scope_exceeds", scopes: [] };
		case "token_may_not_mint":
			return { kind: "forbidden" };
		case "token_grant_invalid":
		case "token_grant_missing":
			return { kind: "grant_invalid" };
		default:
			return { kind: "unavailable" };
	}
}

export function failureMessage(failure: AgentFailure): string {
	switch (failure.kind) {
		case "name_taken":
			return "An agent in this workspace already has that name.";
		case "owner_invalid":
			return "Your account is not an active person in this workspace, so it cannot own an agent here.";
		case "lifecycle_owner_invalid":
			return "This agent's owner is no longer an active person in this workspace. Register a replacement agent.";
		case "disabled":
			return "That agent has been disabled.";
		case "active":
			return "That agent is already active.";
		case "authority_missing":
			return "That agent no longer has authority to restore. Register a new agent instead.";
		case "scope_exceeds":
			return failure.scopes.length === 0
				? "An agent cannot do more than you can. Choose fewer permissions."
				: `An agent cannot do more than you can, and ${listed(
						failure.scopes.map((scope) => agentScopeLabels[scope] ?? scope)
					)} ${failure.scopes.length === 1 ? "is" : "are"} beyond what your role allows. ` +
					"Clear those to carry on, or ask an administrator to raise your role.";
		case "scope_invalid":
			return "One of those permissions is not recognised.";
		case "grant_invalid":
			return "One of those teams is not one you can reach.";
		case "forbidden":
			return "You may not register agents in this workspace.";
		case "unavailable":
			return "We could not reach the server. Try again in a moment.";
	}
}

export function proposalSummary(proposal: AgentProposal): string {
	switch (proposal.action) {
		case "comment":
			return proposal.body ? `“${proposal.body}”` : "a comment";
		case "state_change":
			return proposal.stateName ? `moving it to ${proposal.stateName}` : "a new state";
		case "issue_edit":
			return proposal.title ? `the title “${proposal.title}”` : "an edit";
		case "issue_create":
			return proposal.title
				? `“${proposal.title}”${proposal.teamKey ? ` on ${proposal.teamKey}` : ""}`
				: "a new issue";
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
