import { listingFor, type InboxFilter, type InboxListing } from "$lib/notifications/notifications";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type InboxPageData = { listing: InboxListing; filter: InboxFilter; unread: number };

export const load: PageServerLoad = async ({
	depends,
	locals,
	parent,
	url,
}): Promise<InboxPageData> => {
	const { workspace } = await parent();

	depends(keys.inbox(workspace.id));

	const filter: InboxFilter = url.searchParams.get("filter") === "all" ? "all" : "unread";

	const inbox = await locals.api.GET("/workspaces/{workspaceId}/notifications", {
		params: { path: { workspaceId: workspace.id }, query: { filter } },
	});

	return {
		listing: listingFor(inbox.data, filter),
		filter,
		unread: inbox.data?.unread ?? 0,
	};
};
