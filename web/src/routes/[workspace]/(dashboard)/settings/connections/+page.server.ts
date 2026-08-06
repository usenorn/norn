import type { WorkspaceConnectionListing } from "$lib/account/connections";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type WorkspaceConnectionsPageData = {
	listing: WorkspaceConnectionListing;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<WorkspaceConnectionsPageData> => {
	depends(keys.page(route.id));

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
