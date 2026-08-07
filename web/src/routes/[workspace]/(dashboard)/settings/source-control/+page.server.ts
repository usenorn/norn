import type { SourceControlView } from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SourceControlPageData = {
	view: SourceControlView;
	teams: { id: string; key: string; name: string }[];
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<SourceControlPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const [listing, teams] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/connections", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams", {
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	const reachable = (teams.data ?? []).map((team) => ({
		id: team.id,
		key: team.key,
		name: team.name,
	}));

	if (listing.error) {
		if (listing.response.status === 403) return { view: { kind: "forbidden" }, teams: reachable };
		if (listing.response.status === 503) {
			return { view: { kind: "sealing_unavailable" }, teams: reachable };
		}

		return { view: { kind: "unavailable" }, teams: reachable };
	}

	if (!listing.data || listing.data.length === 0) {
		return { view: { kind: "empty" }, teams: reachable };
	}

	return { view: { kind: "list", connections: listing.data }, teams: reachable };
};
