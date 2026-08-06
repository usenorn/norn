import type { DirectoryView } from "$lib/directory/directory";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type DirectoryPageData = {
	view: DirectoryView;
};

export const load: PageServerLoad = async ({ depends, route, locals, parent }): Promise<DirectoryPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const availability = await locals.api.GET("/workspaces/{workspaceId}/directory/availability", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (availability.error) {
		return { view: { kind: availability.response.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!availability.data?.available) {
		return { view: { kind: "unlicensed" } };
	}

	const settings = await locals.api.GET("/workspaces/{workspaceId}/directory", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (settings.error) {
		return { view: { kind: settings.response.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!settings.data?.connected || !settings.data.connection) {
		return { view: { kind: "disconnected" } };
	}

	const runs = await locals.api.GET("/workspaces/{workspaceId}/directory/runs", {
		params: { path: { workspaceId: workspace.id }, query: { limit: 25 } },
	});

	return {
		view: {
			kind: "connected",
			connection: settings.data.connection,
			scimBaseUrl: settings.data.scimBaseUrl,
			runs: runs.data?.runs ?? [],
		},
	};
};
