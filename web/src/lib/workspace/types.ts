export type WorkspaceContext = {
	existingWorkspace: string | null;
};

export type WorkspaceCreationFailure =
	| { kind: "slug_taken"; slug: string; suggestions: string[] }
	| { kind: "team_key_taken"; key: string; suggestions: string[] }
	| { kind: "signed_out" }
	| { kind: "unavailable" };
