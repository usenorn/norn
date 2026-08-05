import { apiFor } from "$lib/api";
import { listingFor, type InboxFilter, type InboxListing } from "$lib/notifications/notifications";
import type { PageLoad } from "./$types";

export type InboxPageData = { listing: InboxListing; filter: InboxFilter; unread: number };

export const load: PageLoad = async ({ fetch, parent, url }): Promise<InboxPageData> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const filter: InboxFilter = url.searchParams.get("filter") === "all" ? "all" : "unread";

	const inbox = await api.GET("/workspaces/{workspaceId}/notifications", {
		fetch,
		params: { path: { workspaceId: workspace.id }, query: { filter } },
	});

	return {
		listing: listingFor(inbox.data, filter),
		filter,
		unread: inbox.data?.unread ?? 0,
	};
};
