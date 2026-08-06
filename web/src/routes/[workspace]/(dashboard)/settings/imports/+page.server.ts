import { keys } from "$lib/api/keys";
import { runPageSize, type ImportsView } from "$lib/imports/imports";
import type { PageServerLoad } from "./$types";

export type ImportsPageData = {
	view: ImportsView;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<ImportsPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const path = { workspaceId: workspace.id };

	const [sources, runs] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/imports/sources", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/imports", {
			params: { path, query: { limit: runPageSize } },
		}),
	]);

	if (sources.error) {
		if (sources.response.status === 403) return { view: { kind: "forbidden" } };

		return { view: { kind: "unavailable" } };
	}

	return {
		view: {
			kind: "sources",
			sources: sources.data ?? [],
			runs: runs.data?.runs ?? [],
			nextCursor: runs.data?.nextCursor,
		},
	};
};
