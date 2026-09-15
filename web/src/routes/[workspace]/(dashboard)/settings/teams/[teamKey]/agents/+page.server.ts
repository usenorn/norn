import { keys } from "$lib/api/keys";
import { teamOf } from "$lib/team/team-settings";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) return { agents: null };

	const { data } = await locals.api.GET(
		"/workspaces/{workspaceId}/teams/{teamId}/agent-settings",
		{ params: { path: { workspaceId: workspace.id, teamId: team.id } } }
	);

	return { agents: data ?? null };
};
