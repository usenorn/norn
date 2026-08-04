import { settingsFor, type WorkspaceSettings } from "$lib/workspace/settings";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type WorkspaceSettingsData = {
	settings: WorkspaceSettings;
	teams: Team[];
};

export const load: PageLoad = async ({ parent }): Promise<WorkspaceSettingsData> => {
	const { workspace, teams } = await parent();

	return {
		settings: settingsFor(workspace),
		teams: (teams ?? []).filter((team) => team.status === "active"),
	};
};
