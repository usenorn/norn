import { error } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import { issuesListing } from "$lib/issues/listing.server";
import type { IssuesListingData } from "$lib/issues/listing";
import { teamIssuesPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	url,
	cookies,
	params,
	parent,
}): Promise<IssuesListingData> => {
	depends(keys.page(route.id));

	const { workspace, teams, states, now, member } = await parent();

	depends(keys.issues(workspace.id));

	const available = teams ?? [];
	const team = available.find((candidate) => candidate.key === params.teamKey.toUpperCase());
	const previewing =
		import.meta.env.DEV && Boolean(teamIssuesPreviewStates[url.searchParams.get("state") ?? ""]);

	if (!team && !previewing) error(404, "That team does not exist, or you are not on it.");

	return issuesListing({
		api: locals.api,
		workspaceId: workspace.id,
		timezone: workspace.timezone,
		now,
		memberId: member.id,
		teams: available,
		states,
		scope: team ? { kind: "team", team } : { kind: "workspace", views: [] },
		url,
		cookies,
	});
};
