import { keys } from "$lib/api/keys";
import { teamOf } from "$lib/team/team-settings";
import { intakeFor, type IntakeSetting } from "$lib/triage/intake";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) {
		const intake: IntakeSetting = { kind: "unavailable" };

		return { intake };
	}

	const { data, response } = await locals.api.GET(
		"/workspaces/{workspaceId}/teams/{teamId}/intake-address",
		{ params: { path: { workspaceId: workspace.id, teamId: team.id } } }
	);

	return { intake: intakeFor(data, response.status) };
};
