import { keys } from "$lib/api/keys";
import { issuesListing } from "$lib/issues/listing.server";
import type { IssuesListingData } from "$lib/issues/listing";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	url,
	cookies,
	parent,
}): Promise<IssuesListingData> => {
	depends(keys.page(route.id));

	const { workspace, teams, views, states, now, member } = await parent();

	depends(keys.issues(workspace.id));

	return issuesListing({
		api: locals.api,
		workspaceId: workspace.id,
		timezone: workspace.timezone,
		now,
		memberId: member.id,
		teams: teams ?? [],
		states,
		scope: { kind: "workspace", views: views ?? [] },
		url,
		cookies,
	});
};
