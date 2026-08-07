import type { SourceControlRepositoryView } from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SourceControlRepositoryPageData = {
	view: SourceControlRepositoryView;
	teams: { id: string; key: string; name: string }[];
};

export const load: PageServerLoad = async ({
	depends,
	route,
	params,
	locals,
	parent,
}): Promise<SourceControlRepositoryPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();
	const path = { workspaceId: workspace.id, repositoryId: params.repositoryId };

	const [repositories, routes, deliveries, teams] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/repositories", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET(
			"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}/routes",
			{ params: { path } },
		),
		locals.api.GET(
			"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}/deliveries",
			{ params: { path } },
		),
		locals.api.GET("/workspaces/{workspaceId}/teams", {
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	const reachable = (teams.data ?? []).map((team) => ({
		id: team.id,
		key: team.key,
		name: team.name,
	}));

	if (repositories.error) {
		if (repositories.response.status === 403) {
			return { view: { kind: "forbidden" }, teams: reachable };
		}

		return { view: { kind: "unavailable" }, teams: reachable };
	}

	const repository = (repositories.data ?? []).find(
		(one) => one.id === params.repositoryId,
	);

	if (!repository) return { view: { kind: "not_found" }, teams: reachable };

	const connection = await locals.api.GET(
		"/workspaces/{workspaceId}/source-control/connections/{connectionId}",
		{
			params: {
				path: { workspaceId: workspace.id, connectionId: repository.connectionId },
			},
		},
	);

	if (!connection.data) return { view: { kind: "not_found" }, teams: reachable };

	return {
		view: {
			kind: "detail",
			connection: connection.data,
			repository,
			routes: routes.data ?? [],
			deliveries: deliveries.data ?? [],
		},
		teams: reachable,
	};
};
