import { apiFor } from "$lib/api";
import type { CadenceSetting, CycleCadence } from "$lib/cycles/cycles";
import { rosterFor, type TeamRoster } from "$lib/team/members";
import { statesFor, type StateList } from "$lib/team/states";
import { settingsFor, type TeamSettings } from "$lib/team/team-settings";
import { settingFor, type TriageSetting, type TriageSettings } from "$lib/triage/triage";
import type { TeamNotificationSetting } from "$lib/notifications/notifications";
import type { PageLoad } from "./$types";

export type TeamPageData = {
	settings: TeamSettings;
	roster: TeamRoster;
	states: StateList;
	cadence: CadenceSetting;
	triage: TriageSetting;
	notifications: TeamNotificationSetting;
};

const unavailable: TeamPageData = {
	settings: { kind: "unavailable" },
	roster: { kind: "unavailable" },
	states: { kind: "unavailable" },
	cadence: { kind: "unavailable" },
	triage: { kind: "unavailable" },
	notifications: { kind: "unavailable" },
};

export const load: PageLoad = async ({ fetch, params, parent, url}): Promise<TeamPageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();

	if (!teams) return unavailable;

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { ...unavailable, settings: { kind: "not_found" } };

	const path = { workspaceId: workspace.id, teamId: team.id };

	const [members, states, cadence, triage, notifications] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/triage", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/notification-settings", {
			fetch,
			params: { path },
		}),
	]);

	return {
		settings: settingsFor(team),
		roster: rosterFor(members.data),
		states: statesFor(states.data),
		cadence: cadenceFor(cadence.data, cadence.response.status),
		triage: settingFor(triage.data, triage.response.status),
		notifications: notifications.data
			? { kind: "ready", settings: notifications.data }
			: { kind: "unavailable" },
	};
};

function cadenceFor(cadence: CycleCadence | undefined, status: number): CadenceSetting {
	if (cadence) return { kind: "enabled", cadence };
	if (status === 404) return { kind: "disabled" };

	return { kind: "unavailable" };
}
