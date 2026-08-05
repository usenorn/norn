import { apiFor } from "$lib/api";
import type { ConnectionListing } from "$lib/account/connections";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type ConnectionsPageData = {
	listing: ConnectionListing;
	teams: Record<string, Team[]>;
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<ConnectionsPageData> => {
	const api = apiFor(url);
	const { workspaces } = await parent();

	const [connections, ...rosters] = await Promise.all([
		api.GET("/mcp/connections", { fetch }),
		...workspaces.map((workspace) =>
			api.GET("/workspaces/{workspaceId}/teams", {
				fetch,
				params: { path: { workspaceId: workspace.id } },
			})
		),
	]);

	const teams: Record<string, Team[]> = {};

	workspaces.forEach((workspace, index) => {
		teams[workspace.id] = rosters[index]?.data ?? [];
	});

	if (connections.error) {
		return {
			teams,
			listing: { kind: connections.error.status === 403 ? "forbidden" : "unavailable" },
		};
	}

	if (!connections.data || connections.data.length === 0) {
		return { teams, listing: { kind: "empty" } };
	}

	return { teams, listing: { kind: "ready", connections: connections.data } };
};
