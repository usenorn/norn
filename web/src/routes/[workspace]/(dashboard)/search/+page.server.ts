import { listingFor, type SearchListing } from "$lib/search/search";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SearchPageData = { query: string; listing: SearchListing };

export const load: PageServerLoad = async ({ depends, route, locals, parent, url }): Promise<SearchPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();
	const query = url.searchParams.get("q") ?? "";

	if (query.trim() === "") return { query, listing: { kind: "idle" } };

	const results = await locals.api.GET("/workspaces/{workspaceId}/search", {
		params: {
			path: { workspaceId: workspace.id },
			query: { q: query, limit: 25 },
		},
	});

	if (results.error?.status === 422) return { query, listing: { kind: "idle" } };

	return { query, listing: listingFor(results.data) };
};
