import { apiFor } from "$lib/api";
import type { TokenListing } from "$lib/account/tokens";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type TokensPageData = {
	listing: TokenListing;
	teams: Record<string, Team[]>;
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<TokensPageData> => {
	const api = apiFor(url);
	const { workspaces } = await parent();

	// Teams are loaded for every workspace up front so the grant builder can offer them without a
	// request per expansion; a person belongs to few enough workspaces for this to stay cheap.
	const [tokens, ...rosters] = await Promise.all([
		api.GET("/tokens", { fetch }),
		...workspaces.map((workspace) =>
			api.GET("/workspaces/{workspaceId}/teams", {
				fetch,
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
