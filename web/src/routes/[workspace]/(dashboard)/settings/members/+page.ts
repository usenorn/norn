import { apiFor } from "$lib/api";
import { listingFor, memberPageSize, type MemberListing } from "$lib/workspace/members";
import { membersPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

export type MembersData = {
	listing: MemberListing;
	query: string;
};

export const load: PageLoad = async ({ fetch, url, parent }): Promise<MembersData> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const query = url.searchParams.get("q") ?? "";

	if (import.meta.env.DEV && membersPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" }, query };
	}

	const { data } = await api.GET("/workspaces/{workspaceId}/members", {
		fetch,
		params: {
			path: { workspaceId: workspace.id },
			query: { query: query || undefined, limit: memberPageSize },
		},
	});

	return { listing: listingFor(data, query), query };
};
