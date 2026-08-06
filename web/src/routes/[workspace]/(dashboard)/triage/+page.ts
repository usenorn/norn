import { apiFor } from "$lib/api";
import { listingFor, type TriageListing } from "$lib/triage/triage";
import type { Issue } from "$lib/issues/issues";
import type { Member } from "$lib/issues/members";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

const candidateLimit = 200;

export type TriagePageData = {
	listing: TriageListing;
	teams: Team[];
	members: Member[];
	candidates: Issue[];
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<TriagePageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();

	const [queue, members, candidates] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/triage", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/members", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/issues", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { limit: candidateLimit } },
		}),
	]);

	return {
		listing: listingFor(queue.data, teams ?? []),
		teams: teams ?? [],
		members: members.data?.members ?? [],
		candidates: candidates.data?.issues ?? [],
	};
};
