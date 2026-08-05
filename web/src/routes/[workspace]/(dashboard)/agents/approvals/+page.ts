import { apiFor } from "$lib/api";
import type { ProposalQueue } from "$lib/agents/agents";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, parent, url }): Promise<{ queue: ProposalQueue }> => {
	const api = apiFor(url);
	const { workspace } = await parent();

	const { data, error } = await api.GET("/workspaces/{workspaceId}/agent-proposals", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (error) {
		return { queue: { kind: error.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!data || data.length === 0) return { queue: { kind: "empty" } };

	return { queue: { kind: "ready", proposals: data } };
};
