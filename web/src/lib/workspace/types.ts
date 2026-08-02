export type WorkspaceContext = {
	existingWorkspace: string | null;
};

export type WorkspaceCreationFailure = {
	kind: "slug_taken";
	slug: string;
	suggestions: string[];
};
