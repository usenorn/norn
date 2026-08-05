import { apiFor } from "$lib/api";
import { listingFor, type SearchListing } from "$lib/search/search";
import type { PageLoad } from "./$types";

export type SearchPageData = { query: string; listing: SearchListing };

export const load: PageLoad = async ({ fetch, parent, url }): Promise<SearchPageData> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const query = url.searchParams.get("q") ?? "";

	if (query.trim() === "") return { query, listing: { kind: "idle" } };

	const results = await api.GET("/workspaces/{workspaceId}/search", {
		fetch,
		params: {
			path: { workspaceId: workspace.id },
			query: { q: query, limit: 25 },
		},
	});

	if (results.error?.status === 422) return { query, listing: { kind: "idle" } };

	return { query, listing: listingFor(results.data) };
};
