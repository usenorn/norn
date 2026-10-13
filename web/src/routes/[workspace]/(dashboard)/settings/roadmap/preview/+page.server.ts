import { keys } from "$lib/api/keys";
import { readRoadmapPreview, type RoadmapPreviewView } from "$lib/roadmap/admin";
import { roadmapPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	locals,
	parent,
	route,
	url,
}): Promise<{ view: RoadmapPreviewView }> => {
	const { workspace } = await parent();

	depends(keys.page(route.id), keys.roadmap(workspace.id));

	if (import.meta.env.DEV && roadmapPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { view: { kind: "loading" } };
	}

	const path = { workspaceId: workspace.id };

	const [preview, roadmap] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/roadmap/preview", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/roadmap", { params: { path } }),
	]);

	return { view: readRoadmapPreview(preview, roadmap) };
};
