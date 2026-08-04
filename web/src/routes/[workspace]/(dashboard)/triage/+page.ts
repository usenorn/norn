import { apiFor } from "$lib/api";
import { listingFor, type TriageListing } from "$lib/triage/triage";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type TriagePageData = { listing: TriageListing; teams: Team[] };

export const load: PageLoad = async ({ fetch, parent, url }): Promise<TriagePageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();

	const queue = await api.GET("/workspaces/{workspaceId}/triage", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	return { listing: listingFor(queue.data, teams ?? []), teams: teams ?? [] };
};
