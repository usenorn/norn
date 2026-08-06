import type { CadenceSetting, CycleCadence } from "$lib/cycles/cycles";
import { rosterFor, type TeamRoster } from "$lib/team/members";
import { statesFor, type StateList } from "$lib/team/states";
import { settingsFor, type TeamSettings } from "$lib/team/team-settings";
import { settingFor, type TriageSetting, type TriageSettings } from "$lib/triage/triage";
import type { AgentSettings } from "$lib/agents/agents";
import type { TeamNotificationSetting } from "$lib/notifications/notifications";
import type { PageServerLoad } from "./$types";

export type TeamPageData = {
	settings: TeamSettings;
	roster: TeamRoster;
	states: StateList;
	cadence: CadenceSetting;
	triage: TriageSetting;
	agents: AgentSettings | null;
	notifications: TeamNotificationSetting;
};

const unavailable: TeamPageData = {
	settings: { kind: "unavailable" },
	roster: { kind: "unavailable" },
	states: { kind: "unavailable" },
	cadence: { kind: "unavailable" },
	triage: { kind: "unavailable" },
	agents: null,
	notifications: { kind: "unavailable" },
};

export const load: PageServerLoad = async ({ locals, params, parent }): Promise<TeamPageData> => {
	const { workspace, teams } = await parent();

	if (!teams) return unavailable;

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { ...unavailable, settings: { kind: "not_found" } };

	const path = { workspaceId: workspace.id, teamId: team.id };

	const [members, states, cadence, triage, agents, notifications] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/triage", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/agent-settings", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/notification-settings", {
			params: { path },
		}),
	]);

	return {
		settings: settingsFor(team),
		roster: rosterFor(members.data),
		states: statesFor(states.data),
		cadence: cadenceFor(cadence.data, cadence.response.status),
		triage: settingFor(triage.data, triage.response.status),
		agents: agents.data ?? null,
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
