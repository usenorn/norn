import type { SourceControlDetailView } from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SourceControlDetailPageData = {
	view: SourceControlDetailView;
	teams: { id: string; key: string; name: string }[];
};

export const load: PageServerLoad = async ({
	depends,
	route,
	params,
	locals,
	parent,
}): Promise<SourceControlDetailPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const [connection, teams, deliveries] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/connections/{connectionId}", {
			params: { path: { workspaceId: workspace.id, connectionId: params.connectionId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET(
			"/workspaces/{workspaceId}/source-control/connections/{connectionId}/deliveries",
			{ params: { path: { workspaceId: workspace.id, connectionId: params.connectionId } } },
		),
	]);

	const reachable = (teams.data ?? []).map((team) => ({
		id: team.id,
		key: team.key,
		name: team.name,
	}));

	if (connection.error) {
		if (connection.response.status === 403) {
			return { view: { kind: "forbidden" }, teams: reachable };
		}

		if (connection.response.status === 404) {
			return { view: { kind: "not_found" }, teams: reachable };
		}

		return { view: { kind: "unavailable" }, teams: reachable };
	}

	if (!connection.data) return { view: { kind: "not_found" }, teams: reachable };

	return {
		view: {
			kind: "detail",
			connection: connection.data,
			links: [],
			deliveries: deliveries.data ?? [],
		},
		teams: reachable,
	};
};
