import { keys } from "$lib/api/keys";
import { rosterFor, type TeamRoster } from "$lib/team/members";
import { teamOf } from "$lib/team/team-settings";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) {
		const roster: TeamRoster = { kind: "unavailable" };

		return { roster };
	}

	const members = await locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", {
		params: { path: { workspaceId: workspace.id, teamId: team.id } },
	});

	return { roster: rosterFor(members.data) };
};
