import type { SourceControlDetailView } from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SourceControlDetailPageData = {
	view: SourceControlDetailView;
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

	const [connection, repositories] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/connections/{connectionId}", {
			params: { path: { workspaceId: workspace.id, connectionId: params.connectionId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/source-control/repositories", {
			params: {
				path: { workspaceId: workspace.id },
				query: { connectionId: params.connectionId },
			},
		}),
	]);

	if (connection.error) {
		if (connection.response.status === 403) return { view: { kind: "forbidden" } };
		if (connection.response.status === 404) return { view: { kind: "not_found" } };

		return { view: { kind: "unavailable" } };
	}

	if (!connection.data) return { view: { kind: "not_found" } };

	return {
		view: {
			kind: "detail",
			connection: connection.data,
			repositories: repositories.data ?? [],
		},
	};
};
