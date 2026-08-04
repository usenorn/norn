import { listingFor, type TeamListing } from "$lib/team/teams";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ parent }): Promise<{ listing: TeamListing }> => {
	const { teams } = await parent();

	return { listing: listingFor(teams) };
};
