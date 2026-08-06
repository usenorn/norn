import type { TokenListing } from "$lib/account/tokens";
import { keys } from "$lib/api/keys";
import type { Team } from "$lib/team/teams";
import type { PageServerLoad } from "./$types";

export type TokensPageData = {
	listing: TokenListing;
	teams: Record<string, Team[]>;
};

export const load: PageServerLoad = async ({ depends, route, locals, parent }): Promise<TokensPageData> => {
	depends(keys.page(route.id));

	const { workspaces } = await parent();

	const [tokens, ...rosters] = await Promise.all([
		locals.api.GET("/tokens"),
		...workspaces.map((workspace) =>
			locals.api.GET("/workspaces/{workspaceId}/teams", {
				params: { path: { workspaceId: workspace.id } },
			})
		),
	]);

	const teams: Record<string, Team[]> = {};

	workspaces.forEach((workspace, index) => {
		teams[workspace.id] = rosters[index]?.data ?? [];
	});

	if (tokens.error) {
		return {
			teams,
			listing: { kind: tokens.error.status === 403 ? "forbidden" : "unavailable" },
		};
	}

	if (!tokens.data || tokens.data.length === 0) return { teams, listing: { kind: "empty" } };

	return { teams, listing: { kind: "ready", tokens: tokens.data } };
};
