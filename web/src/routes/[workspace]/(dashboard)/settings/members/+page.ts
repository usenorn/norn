import { apiFor } from "$lib/api";
import type { WorkspaceAPIToken } from "$lib/account/tokens";
import { listingFor, memberPageSize, type MemberListing } from "$lib/workspace/members";
import { membersPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

export type MembersData = {
	listing: MemberListing;
	query: string;
	linked: string[];
	tokens: WorkspaceAPIToken[] | null;
};

export const load: PageLoad = async ({ fetch, url, parent }): Promise<MembersData> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const query = url.searchParams.get("q") ?? "";

	if (import.meta.env.DEV && membersPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" }, query, linked: [], tokens: null };
	}

	const [members, identities, tokens] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/members", {
			fetch,
			params: {
				path: { workspaceId: workspace.id },
				query: { query: query || undefined, limit: memberPageSize },
			},
		}),
		api.GET("/workspaces/{workspaceId}/sso/identities", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/tokens", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	return {
		listing: listingFor(members.data, query),
		query,
		linked: (identities.data ?? []).map((identity) => identity.accountId),
		tokens: tokens.error ? null : (tokens.data ?? []),
	};
};
