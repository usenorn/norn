import { keys } from "#lib/api/keys.js";
import { readRoadmapPreviewEntry, type RoadmapPreviewEntryView } from "#lib/roadmap/admin.js";
import { roadmapPreviewEntryStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	locals,
	params,
	parent,
	route,
	url,
}): Promise<{ view: RoadmapPreviewEntryView }> => {
	const { workspace } = await parent();

	depends(keys.page(route.id), keys.roadmap(workspace.id));

	if (import.meta.env.DEV && roadmapPreviewEntryStates[url.searchParams.get("state") ?? ""]) {
		return { view: { kind: "loading" } };
	}

	const path = { workspaceId: workspace.id };

	const [preview, item] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/roadmap/preview", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/roadmap/preview/entries/{entryId}", {
			params: { path: { ...path, entryId: params.entryId } },
		}),
	]);

	return { view: readRoadmapPreviewEntry(preview, item) };
};
