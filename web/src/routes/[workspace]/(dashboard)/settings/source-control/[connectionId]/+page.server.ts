import type { SourceControlDetailView } from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SourceControlDetailPageData = {
	view: SourceControlDetailView;
	/** Where the installation is administered, so the copy about it has somewhere to go. */
	installUrl: string;
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

	const [connection, repositories, application] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/connections/{connectionId}", {
			params: { path: { workspaceId: workspace.id, connectionId: params.connectionId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/source-control/repositories", {
			params: {
				path: { workspaceId: workspace.id },
				query: { connectionId: params.connectionId },
			},
		}),
		locals.api.GET("/workspaces/{workspaceId}/source-control/application", {
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	// Where the installation is administered, so "change what this reaches" has somewhere to go.
	const installUrl = application.data?.installUrl ?? "";

	if (connection.error) {
		if (connection.response.status === 403) return { view: { kind: "forbidden" }, installUrl };
		if (connection.response.status === 404) return { view: { kind: "not_found" }, installUrl };

		return { view: { kind: "unavailable" }, installUrl };
	}

	if (!connection.data) return { view: { kind: "not_found" }, installUrl };

	return {
		view: {
			kind: "detail",
			connection: connection.data,
			repositories: repositories.data ?? [],
		},
		installUrl,
	};
};
