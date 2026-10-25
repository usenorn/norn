import type { Reroute } from "@sveltejs/kit/hooks";
import { PUBLIC_ROADMAPS_DOMAIN } from "$app/env/public";
import { onRoadmapHost, publicRoadmapPath } from "#lib/roadmap/host.js";

export const reroute: Reroute = ({ url }) => {
	if (onRoadmapHost(url.hostname, PUBLIC_ROADMAPS_DOMAIN)) {
		return publicRoadmapPath(url.pathname);
	}
};
