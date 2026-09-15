import { settingsFor, type TeamSettings } from "$lib/team/team-settings";
import { managesTeams } from "$lib/workspace/members";
import type { LayoutServerLoad } from "./$types";

export const load: LayoutServerLoad = async ({ params, parent }) => {
	const { teams, members, member } = await parent();
	const readOnly = !managesTeams(members, member.id);

	if (!teams) {
		const settings: TeamSettings = { kind: "unavailable" };

		return { settings, readOnly };
	}

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());
	const settings: TeamSettings = team ? settingsFor(team) : { kind: "not_found" };

	return { settings, readOnly };
};
