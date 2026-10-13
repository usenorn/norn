import { keys } from "$lib/api/keys";
import { readPublicRoadmap, type PublicRoadmapView } from "$lib/roadmap/public-roadmap";
import { publicRoadmapPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	locals,
	route,
	url,
}): Promise<{ view: PublicRoadmapView }> => {
	depends(keys.page(route.id));

	if (import.meta.env.DEV && publicRoadmapPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { view: { kind: "loading" } };
	}

	const answer = await locals.api.GET("/public/roadmap");

	return { view: readPublicRoadmap(answer, locals.correlationId) };
};
