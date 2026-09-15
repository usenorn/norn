import { keys } from "$lib/api/keys";
import { statesFor, type StateList } from "$lib/team/states";
import { teamOf } from "$lib/team/team-settings";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) {
		const states: StateList = { kind: "unavailable" };

		return { states };
	}

	const { data } = await locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
		params: { path: { workspaceId: workspace.id, teamId: team.id } },
	});

	return { states: statesFor(data) };
};
