import { keys } from "$lib/api/keys";
import { statesFor } from "$lib/team/states";
import { teamOf } from "$lib/team/team-settings";
import type { PageServerLoad } from "./$types";

const branchTemplate = "{handle}/{reference}-{title}";

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace, settings } = await parent();
	const team = teamOf(settings);

	if (!team) {
		return {
			rules: [],
			sourceControlSettings: { teamId: "", branchTemplate },
			states: statesFor(undefined),
		};
	}

	const path = { workspaceId: workspace.id, teamId: team.id };

	const [rules, scmSettings, states] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/source-control/rules", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/source-control/settings", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", { params: { path } }),
	]);

	return {
		rules: rules.data ?? [],
		sourceControlSettings: scmSettings.data ?? { teamId: team.id, branchTemplate },
		states: statesFor(states.data),
	};
};
