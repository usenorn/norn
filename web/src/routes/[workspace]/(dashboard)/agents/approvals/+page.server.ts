import type { ProposalQueue } from "$lib/agents/agents";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	locals,
	parent,
}): Promise<{ queue: ProposalQueue }> => {
	const { workspace } = await parent();

	const { data, error } = await locals.api.GET("/workspaces/{workspaceId}/agent-proposals", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (error) {
		return { queue: { kind: error.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!data || data.length === 0) return { queue: { kind: "empty" } };

	return { queue: { kind: "ready", proposals: data } };
};
