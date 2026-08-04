import { apiFor } from "$lib/api";
import type { CycleListing } from "$lib/cycles/cycles";
import { teamCyclesPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

export type TeamCyclesData = {
	listing: CycleListing;
};

export const load: PageLoad = async ({ fetch, params, parent, url }): Promise<TeamCyclesData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();

	if (import.meta.env.DEV && teamCyclesPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" } };
	}

	if (!teams) return { listing: { kind: "unavailable" } };

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { listing: { kind: "not_found" } };

	const [cycles, cadence] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/cycles", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { teamId: team.id } },
		}),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/cycle-cadence", {
			fetch,
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
