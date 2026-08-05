import { apiFor } from "$lib/api";
import type { DirectoryView } from "$lib/directory/directory";
import type { PageLoad } from "./$types";

export type DirectoryPageData = {
	view: DirectoryView;
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<DirectoryPageData> => {
	const api = apiFor(url);
	const { workspace } = await parent();

	const availability = await api.GET("/workspaces/{workspaceId}/directory/availability", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (availability.error) {
		return { view: { kind: availability.response.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!availability.data?.available) {
		return { view: { kind: "unlicensed", holder: availability.data?.holder } };
	}

	const settings = await api.GET("/workspaces/{workspaceId}/directory", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (settings.error) {
		return { view: { kind: settings.response.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!settings.data?.connected || !settings.data.connection) {
		return { view: { kind: "disconnected" } };
	}

	const runs = await api.GET("/workspaces/{workspaceId}/directory/runs", {
		fetch,
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
