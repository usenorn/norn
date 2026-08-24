import { keys } from "$lib/api/keys";
import type { ReviewQueue } from "$lib/executions/reviews";
import { reviewPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type ReviewsPageData = { queue: ReviewQueue };

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
	url,
}): Promise<ReviewsPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	if (import.meta.env.DEV && reviewPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { queue: { kind: "loading" } };
	}

	depends(keys.reviews(workspace.id));

	const waiting = await locals.api.GET("/workspaces/{workspaceId}/executions", {
		params: { path: { workspaceId: workspace.id }, query: { state: ["awaiting_review"] } },
	});

	if (!waiting.data) return { queue: { kind: "unavailable" } };

	return { queue: { kind: "ready", runs: waiting.data } };
};
