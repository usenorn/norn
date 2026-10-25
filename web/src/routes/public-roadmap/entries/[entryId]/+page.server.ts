import { keys } from "#lib/api/keys.js";
import { readPublicRoadmapEntry, type PublicRoadmapEntryView } from "#lib/roadmap/public-roadmap.js";
import { publicRoadmapEntryPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	locals,
	params,
	route,
	url,
}): Promise<{ view: PublicRoadmapEntryView }> => {
	depends(keys.page(route.id));

	if (import.meta.env.DEV && publicRoadmapEntryPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { view: { kind: "loading" } };
	}

	const [roadmap, item] = await Promise.all([
		locals.api.GET("/public/roadmap"),
		locals.api.GET("/public/roadmap/entries/{entryId}", {
			params: { path: { entryId: params.entryId } },
		}),
	]);

	return { view: readPublicRoadmapEntry(roadmap, item, locals.correlationId) };
};
