import type { WebhookDetailView } from "$lib/webhooks/webhooks";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

const pageSize = 25;

export type WebhookDetailPageData = {
	view: WebhookDetailView;
};

export const load: PageServerLoad = async ({ depends, route, locals, params, parent }): Promise<WebhookDetailPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const path = { workspaceId: workspace.id, webhookId: params.webhookId };

	const [webhook, deliveries] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/webhooks/{webhookId}", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/webhooks/{webhookId}/deliveries", {
			params: { path, query: { limit: pageSize } },
		}),
	]);

	if (webhook.error) {
		if (webhook.response.status === 403) return { view: { kind: "forbidden" } };
		if (webhook.response.status === 404) return { view: { kind: "not_found" } };

		return { view: { kind: "unavailable" } };
	}

	if (!webhook.data) return { view: { kind: "not_found" } };

	const settled = deliveries.data?.deliveries.find((delivery) => delivery.state === "failed");

	const failure =
		webhook.data.enabled || !settled
			? undefined
			: await locals.api.GET("/workspaces/{workspaceId}/webhooks/{webhookId}/deliveries/{deliveryId}", {
					params: { path: { ...path, deliveryId: settled.id } },
				});

	return {
		view: {
			kind: "detail",
			webhook: webhook.data,
			deliveries: deliveries.data?.deliveries ?? [],
			nextCursor: deliveries.data?.nextCursor,
			lastAttempt: failure?.data?.attempts.at(-1),
		},
	};
};
