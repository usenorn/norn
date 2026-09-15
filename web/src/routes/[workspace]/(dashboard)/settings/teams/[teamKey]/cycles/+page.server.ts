import { keys } from "$lib/api/keys";
import type { CadenceSetting, CycleCadence } from "$lib/cycles/cycles";
import { teamOf } from "$lib/team/team-settings";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) {
		const cadence: CadenceSetting = { kind: "unavailable" };

		return { cadence };
	}

	const { data, response } = await locals.api.GET(
		"/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence",
		{ params: { path: { workspaceId: workspace.id, teamId: team.id } } }
	);

	return { cadence: cadenceFor(data, response.status) };
};

function cadenceFor(cadence: CycleCadence | undefined, status: number): CadenceSetting {
	if (cadence) return { kind: "enabled", cadence };
	if (status === 404) return { kind: "disabled" };

	return { kind: "unavailable" };
}
