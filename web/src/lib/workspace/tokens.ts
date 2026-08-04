import type { components, operations } from "$lib/api/dashboard.gen";

export type APIToken = components["schemas"]["APIToken"];
export type APIScope = components["schemas"]["APIScope"];

type MintResponses = operations["mintWorkspaceAPIToken"]["responses"];

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
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const scopeCatalog: APIScope[] = [
	"workspace:read",
	"membership:read",
	"invitation:read",
	"team:read",
	"team:manage",
	"team_membership:read",
	"team_membership:manage",
	"issue:read",
	"issue:manage",
];

export const scopeLabels: Record<string, string> = {
	"workspace:read": "Read the workspace",
	"membership:read": "Read members",
	"invitation:read": "Read invitations",
	"team:read": "Read teams",
	"team:manage": "Create and change teams",
	"team_membership:read": "Read team rosters",
	"team_membership:manage": "Change team rosters",
	"issue:read": "Read issues",
	"issue:manage": "Raise issues",
};

function coded(problem: MintProblem): problem is CodedMintProblem {
	return "code" in problem;
}

export function mintFailure(problem: MintProblem): TokenFailure {
	if (problem.status === 403) return { kind: "forbidden" };

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
	}
}

export function failureMessage(failure: TokenFailure): string {
	switch (failure.kind) {
		case "name_taken":
			return "You already have a live token with that name.";
		case "scope_invalid":
			return "One of those permissions is not recognised.";
		case "scope_exceeds":
			return "A token cannot do more than you can. Choose fewer permissions.";
		case "may_not_mint":
			return "Tokens cannot mint other tokens.";
		case "forbidden":
			return "You may not manage tokens in this workspace.";
		case "unavailable":
			return "We could not reach the server. Try again in a moment.";
	}
}
