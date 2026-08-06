import type { ProposalQueue } from "$lib/agents/agents";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<{ queue: ProposalQueue }> => {
	depends(keys.page(route.id));

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
