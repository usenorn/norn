import { apiFor } from "$lib/api";
import type { WorkspaceConnectionListing } from "$lib/account/connections";
import type { PageLoad } from "./$types";

export type WorkspaceConnectionsPageData = {
	listing: WorkspaceConnectionListing;
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<WorkspaceConnectionsPageData> => {
	const api = apiFor(url);
	const { workspace } = await parent();

	const connections = await api.GET("/workspaces/{workspaceId}/mcp-connections", {
		fetch,
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
