import { apiFor } from "$lib/api";
import type { TokenListing } from "$lib/workspace/tokens";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, parent, url}): Promise<{ listing: TokenListing }> => {
	const api = apiFor(url);

	const { workspace } = await parent();

	const { data, error } = await api.GET("/workspaces/{workspaceId}/tokens", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (error) {
		return { listing: { kind: error.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!data || data.length === 0) return { listing: { kind: "empty" } };

	return { listing: { kind: "ready", tokens: data } };
};
