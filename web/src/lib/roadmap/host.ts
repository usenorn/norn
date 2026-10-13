export const publicRoadmapRoute = "/public-roadmap";

const workspaceSlug = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const workspaceSlugMinLength = 2;
const workspaceSlugMaxLength = 40;

function trimmed(name: string): string {
	return name.trim().toLowerCase().replace(/\.$/, "");
}

export function onRoadmapHost(hostname: string, domain: string | undefined): boolean {
	const suffix = domain ? trimmed(domain) : "";

	if (!suffix) return false;

	const host = trimmed(hostname);

	if (!host.endsWith(`.${suffix}`)) return false;

	const slug = host.slice(0, -(suffix.length + 1));

	return (
		slug.length >= workspaceSlugMinLength &&
		slug.length <= workspaceSlugMaxLength &&
		workspaceSlug.test(slug)
	);
}

export function publicRoadmapPath(pathname: string): string {
	return pathname === "/" ? publicRoadmapRoute : `${publicRoadmapRoute}${pathname}`;
}

export function isPublicRoadmapRoute(routeId: string | null): boolean {
	return routeId === publicRoadmapRoute || (routeId?.startsWith(`${publicRoadmapRoute}/`) ?? false);
}
