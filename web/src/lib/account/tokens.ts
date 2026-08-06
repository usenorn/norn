import type { components, operations } from "$lib/api/dashboard.gen";

export type APIToken = components["schemas"]["APIToken"];
export type APIScope = components["schemas"]["APIScope"];
export type APITokenGrant = components["schemas"]["APITokenGrant"];
export type WorkspaceAPIToken = components["schemas"]["WorkspaceAPIToken"];

type MintResponses = operations["mintAPIToken"]["responses"];

type CodedMintProblem = MintResponses[409]["content"]["application/problem+json"];

export type MintProblem =
	MintResponses[403 | 409 | 422 | 500]["content"]["application/problem+json"];

export type TokenListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; tokens: APIToken[] }
	| { kind: "minted"; tokens: APIToken[]; token: APIToken; value: string }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type TokenFailure =
	| { kind: "name_taken" }
	| { kind: "scope_invalid" }
	| { kind: "scope_exceeds" }
	| { kind: "may_not_mint" }
	| { kind: "grant_invalid" }
	| { kind: "grant_missing" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type MintOutcome = { kind: "minted"; token: APIToken; value: string } | TokenFailure;

export type ScopeGroup = { resource: string; title: string; scopes: APIScope[] };

export const scopeGroups: ScopeGroup[] = [
	{ resource: "issue", title: "Issues", scopes: ["issue:read", "issue:manage"] },
	{ resource: "project", title: "Projects", scopes: ["project:read", "project:manage"] },
	{ resource: "label", title: "Labels", scopes: ["label:read", "label:manage"] },
	{ resource: "team", title: "Teams", scopes: ["team:read", "team:manage"] },
	{
		resource: "team_membership",
		title: "Team rosters",
		scopes: ["team_membership:read", "team_membership:manage"],
	},
	{ resource: "membership", title: "Members", scopes: ["membership:read", "membership:manage"] },
	{
		resource: "invitation",
		title: "Invitations",
		scopes: ["invitation:read", "invitation:manage"],
	},
	{ resource: "workspace", title: "Workspace", scopes: ["workspace:read", "workspace:update"] },
];

export const scopeCatalog: APIScope[] = scopeGroups.flatMap((group) => group.scopes);

export const scopeLabels: Record<string, string> = {
	"workspace:read": "Read workspace settings",
	"workspace:update": "Change workspace settings",
	"membership:read": "Read members",
	"membership:manage": "Add and remove members",
	"invitation:read": "Read invitations",
	"invitation:manage": "Send and revoke invitations",
	"team:read": "Read teams",
	"team:manage": "Create and change teams",
	"team_membership:read": "Read team rosters",
	"team_membership:manage": "Change team rosters",
	"issue:read": "Read issues",
	"issue:manage": "Raise and change issues",
	"label:read": "Read labels",
	"label:manage": "Create and change labels",
	"project:read": "Read projects",
	"project:manage": "Create and change projects",
	"comment:read": "Read comments",
	"comment:manage": "Leave comments",
	"notification:read": "Read your inbox",
	"notification:manage": "Mark your inbox as read",
};

export function writes(scope: APIScope): boolean {
	return !scope.endsWith(":read");
}

function coded(problem: MintProblem): problem is CodedMintProblem {
	return "code" in problem;
}

export function mintFailure(problem: MintProblem): TokenFailure {
	if (problem.status === 403 && !coded(problem)) return { kind: "forbidden" };

	if (!coded(problem)) return { kind: "unavailable" };

	switch (problem.code) {
		case "token_name_taken":
			return { kind: "name_taken" };
		case "token_scope_invalid":
			return { kind: "scope_invalid" };
		case "token_scope_exceeds":
			return { kind: "scope_exceeds" };
		case "token_may_not_mint":
			return { kind: "may_not_mint" };
		case "token_grant_invalid":
			return { kind: "grant_invalid" };
		case "token_grant_missing":
			return { kind: "grant_missing" };
	}
}

export function failureMessage(failure: TokenFailure): string {
	switch (failure.kind) {
		case "name_taken":
			return "You already have a live token with that name.";
		case "scope_invalid":
			return "One of those permissions is not recognised.";
		case "scope_exceeds":
			return "A token cannot do more than you can. You do not have every chosen permission in every workspace you picked.";
		case "may_not_mint":
			return "Tokens cannot mint other tokens.";
		case "grant_invalid":
			return "One of those workspaces or teams is not one you can reach.";
		case "grant_missing":
			return "Choose at least one workspace for this token to reach.";
		case "forbidden":
			return "You may not manage tokens.";
		case "unavailable":
			return "We could not reach the server. Try again in a moment.";
	}
}

export const expiringSoonDays = 30;

export const defaultExpiryDays = 90;

export function daysUntil(expiresAt: string | undefined, now: string): number | null {
	if (!expiresAt) return null;

	const remaining = Date.parse(expiresAt) - Date.parse(now);

	return Math.floor(remaining / 86_400_000);
}
