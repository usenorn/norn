import { apiFor } from "$lib/api";
import { rosterFor, type TeamRoster } from "$lib/team/members";
import { statesFor, type StateList } from "$lib/team/states";
import { settingsFor, type TeamSettings } from "$lib/team/team-settings";
import type { PageLoad } from "./$types";

export type TeamPageData = {
	settings: TeamSettings;
	roster: TeamRoster;
	states: StateList;
};

const unavailable: TeamPageData = {
	settings: { kind: "unavailable" },
	roster: { kind: "unavailable" },
	states: { kind: "unavailable" },
};

export const load: PageLoad = async ({ fetch, params, parent, url}): Promise<TeamPageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();

	if (!teams) return unavailable;

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { ...unavailable, settings: { kind: "not_found" } };

	const path = { workspaceId: workspace.id, teamId: team.id };

	const [members, states] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", { fetch, params: { path } }),
	]);

	return {
		settings: settingsFor(team),
		roster: rosterFor(members.data),
		states: statesFor(states.data),
	};
};
