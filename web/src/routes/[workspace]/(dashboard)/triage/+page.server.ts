import { listingFor, type TriageListing } from "$lib/triage/triage";
import { keys } from "$lib/api/keys";
import { candidateOf, type IssueCandidate } from "$lib/issues/issues";
import type { Member } from "$lib/issues/members";
import type { Team } from "$lib/team/teams";
import type { PageServerLoad } from "./$types";

const candidateLimit = 200;

export type TriagePageData = {
	listing: TriageListing;
	teams: Team[];
	members: Member[];
	candidates: IssueCandidate[];
};

export const load: PageServerLoad = async ({
	depends,
	locals,
	parent,
}): Promise<TriagePageData> => {
	const { workspace, teams } = await parent();

	depends(keys.triage(workspace.id));

	const [queue, members, candidates] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/triage", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/members", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues", {
			params: { path: { workspaceId: workspace.id }, query: { limit: candidateLimit } },
		}),
	]);

	return {
		listing: listingFor(queue.data, teams ?? []),
		teams: teams ?? [],
		members: members.data?.members ?? [],
		candidates: (candidates.data?.issues ?? []).map(candidateOf),
	};
};
