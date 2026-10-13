import { env } from "$env/dynamic/public";
import type { Reroute } from "@sveltejs/kit";
import { onRoadmapHost, publicRoadmapPath } from "$lib/roadmap/host";

export const reroute: Reroute = ({ url }) => {
	if (onRoadmapHost(url.hostname, env.PUBLIC_ROADMAPS_DOMAIN)) {
		return publicRoadmapPath(url.pathname);
	}
};
