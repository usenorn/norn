import type { CycleListing } from "$lib/cycles/cycles";
import { keys } from "$lib/api/keys";
import { teamCyclesPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type TeamCyclesData = {
	listing: CycleListing;
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<TeamCyclesData> => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();

	depends(keys.cycles(workspace.id));

	if (import.meta.env.DEV && teamCyclesPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" } };
	}

	if (!teams) return { listing: { kind: "unavailable" } };

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { listing: { kind: "not_found" } };

	const [cycles, cadence] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/cycles", {
			params: { path: { workspaceId: workspace.id }, query: { teamId: team.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence", {
			params: { path: { workspaceId: workspace.id, teamId: team.id } },
		}),
	]);

	if (cycles.error || !cycles.data) return { listing: { kind: "unavailable" } };

	if (cadence.error && cycles.data.length === 0) {
		return { listing: { kind: "disabled", teamKey: team.key } };
	}

	return {
		listing: {
			kind: "ready",
			teamKey: team.key,
			teamName: team.name,
			cycles: cycles.data,
		},
	};
};
