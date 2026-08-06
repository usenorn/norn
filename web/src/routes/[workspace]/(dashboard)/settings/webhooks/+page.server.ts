import type { WebhooksView } from "$lib/webhooks/webhooks";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type WebhooksPageData = {
	view: WebhooksView;
};

export const load: PageServerLoad = async ({ depends, route, locals, parent }): Promise<WebhooksPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const listing = await locals.api.GET("/workspaces/{workspaceId}/webhooks", {
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
