import { listingFor, type ViewListing } from "$lib/views/views";
import type { Team } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export type ViewsPageData = { listing: ViewListing; teams: Team[] };

export const load: PageLoad = async ({ parent }): Promise<ViewsPageData> => {
	const { views, teams } = await parent();

	return { listing: listingFor(views), teams: teams ?? [] };
};
