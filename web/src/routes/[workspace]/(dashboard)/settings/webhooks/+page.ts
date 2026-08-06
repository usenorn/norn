import { apiFor } from "$lib/api";
import type { WebhooksView } from "$lib/webhooks/webhooks";
import type { PageLoad } from "./$types";

export type WebhooksPageData = {
	view: WebhooksView;
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<WebhooksPageData> => {
	const api = apiFor(url);
	const { workspace } = await parent();

	const listing = await api.GET("/workspaces/{workspaceId}/webhooks", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (listing.error) {
		if (listing.response.status === 403) return { view: { kind: "forbidden" } };
		if (listing.response.status === 503) return { view: { kind: "signing_unavailable" } };

		return { view: { kind: "unavailable" } };
	}

	if (!listing.data || listing.data.length === 0) return { view: { kind: "empty" } };

	return { view: { kind: "list", webhooks: listing.data } };
};
