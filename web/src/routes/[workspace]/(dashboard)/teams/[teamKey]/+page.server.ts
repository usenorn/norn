import { keys } from "$lib/api/keys";
import type { TeamOverview } from "$lib/team/teams";
import { teamOverviewPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type TeamOverviewData = {
	overview: TeamOverview;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	params,
	parent,
	url,
}): Promise<TeamOverviewData> => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();

	depends(keys.workspaceScope(workspace.id));

	if (import.meta.env.DEV && teamOverviewPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { overview: { kind: "loading" } };
	}

	if (!teams) return { overview: { kind: "unavailable" } };

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { overview: { kind: "not_found" } };

	return { overview: { kind: "ready", team } };
};
