import type { ConnectionListing } from "$lib/account/connections";
import { keys } from "$lib/api/keys";
import type { Team } from "$lib/team/teams";
import type { PageServerLoad } from "./$types";

export type ConnectionsPageData = {
	listing: ConnectionListing;
	teams: Record<string, Team[]>;
};

export const load: PageServerLoad = async ({ depends, route, locals, parent }): Promise<ConnectionsPageData> => {
	depends(keys.page(route.id));

	const { workspaces } = await parent();

	const [connections, ...rosters] = await Promise.all([
		locals.api.GET("/mcp/connections"),
		...workspaces.map((workspace) =>
			locals.api.GET("/workspaces/{workspaceId}/teams", {
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
