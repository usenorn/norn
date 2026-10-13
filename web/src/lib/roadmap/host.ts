export const publicRoadmapRoute = "/public-roadmap";

export function onRoadmapHost(hostname: string, domain: string | undefined): boolean {
	const suffix = domain?.trim().toLowerCase().replace(/\.$/, "");

	if (!suffix) return false;

	const host = hostname.toLowerCase().replace(/\.$/, "");

	return host === suffix || host.endsWith(`.${suffix}`);
}

export function publicRoadmapPath(pathname: string): string {
	return pathname === "/" ? publicRoadmapRoute : `${publicRoadmapRoute}${pathname}`;
}

export function isPublicRoadmapRoute(routeId: string | null): boolean {
	return routeId === publicRoadmapRoute || (routeId?.startsWith(`${publicRoadmapRoute}/`) ?? false);
}
