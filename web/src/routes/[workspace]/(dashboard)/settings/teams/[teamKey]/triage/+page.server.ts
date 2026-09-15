import { keys } from "$lib/api/keys";
import { teamOf } from "$lib/team/team-settings";
import { settingFor, type TriageSetting } from "$lib/triage/triage";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) {
		const triage: TriageSetting = { kind: "unavailable" };

		return { triage };
	}

	const { data, response } = await locals.api.GET(
		"/workspaces/{workspaceId}/teams/{teamId}/triage",
		{ params: { path: { workspaceId: workspace.id, teamId: team.id } } }
	);

	return { triage: settingFor(data, response.status) };
};
