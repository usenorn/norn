import type { WorkspaceConnectionListing } from "$lib/account/connections";
import type { PageServerLoad } from "./$types";

export type WorkspaceConnectionsPageData = {
	listing: WorkspaceConnectionListing;
};

export const load: PageServerLoad = async ({
	locals,
	parent,
}): Promise<WorkspaceConnectionsPageData> => {
	const { workspace } = await parent();

	const connections = await locals.api.GET("/workspaces/{workspaceId}/mcp-connections", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (connections.error) {
		return {
			listing: { kind: connections.error.status === 403 ? "forbidden" : "unavailable" },
		};
	}

	if (!connections.data || connections.data.length === 0) {
		return { listing: { kind: "empty" } };
	}

	return { listing: { kind: "ready", connections: connections.data } };
};
