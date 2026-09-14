import { keys } from "$lib/api/keys";
import { rosterFor, type TeamRoster } from "$lib/team/members";
import type { TeamOverview } from "$lib/team/teams";
import { teamOverviewPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type TeamHomeTab = "overview" | "members";

export type TeamOverviewData = {
	overview: TeamOverview;
	roster: TeamRoster;
	tab: TeamHomeTab;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<TeamOverviewData> => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();

	depends(keys.workspaceScope(workspace.id));

	const tab: TeamHomeTab = url.searchParams.get("tab") === "members" ? "members" : "overview";

	if (import.meta.env.DEV && teamOverviewPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { overview: { kind: "loading" }, roster: { kind: "loading" }, tab };
	}

	if (!teams) return { overview: { kind: "unavailable" }, roster: { kind: "unavailable" }, tab };

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { overview: { kind: "not_found" }, roster: { kind: "unavailable" }, tab };

	const members = await locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", {
		params: { path: { workspaceId: workspace.id, teamId: team.id } },
	});

	return {
		overview: { kind: "ready", team },
		roster: rosterFor(members.error ? undefined : members.data),
		tab,
	};
};
