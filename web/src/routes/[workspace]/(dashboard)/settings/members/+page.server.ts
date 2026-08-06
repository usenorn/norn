import type { WorkspaceAPIToken } from "$lib/account/tokens";
import { keys } from "$lib/api/keys";
import { listingFor, memberPageSize, type MemberListing } from "$lib/workspace/members";
import { membersPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type MembersData = {
	listing: MemberListing;
	query: string;
	linked: string[];
	tokens: WorkspaceAPIToken[] | null;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	url,
	parent,
}): Promise<MembersData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.members(workspace.id));

	const query = url.searchParams.get("q") ?? "";

	if (import.meta.env.DEV && membersPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" }, query, linked: [], tokens: null };
	}

	const [members, identities, tokens] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/members", {
			params: {
				path: { workspaceId: workspace.id },
				query: { query: query || undefined, limit: memberPageSize },
			},
		}),
		locals.api.GET("/workspaces/{workspaceId}/sso/identities", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/tokens", {
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
