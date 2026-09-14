import type { ProjectListing } from "$lib/projects/projects";
import { keys } from "$lib/api/keys";
import { projectsPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type ProjectsData = { listing: ProjectListing };

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
	url,
}): Promise<ProjectsData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.projects(workspace.id));

	if (import.meta.env.DEV && projectsPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" } };
	}

	const archived = url.searchParams.get("archived") === "1";

	const projects = await locals.api.GET("/workspaces/{workspaceId}/projects", {
		params: { path: { workspaceId: workspace.id }, query: { archived } },
	});

	if (projects.error || !projects.data) return { listing: { kind: "unavailable" } };

	if (projects.data.length === 0) return { listing: { kind: archived ? "no_matches" : "empty" } };

	return { listing: { kind: "ready", projects: projects.data } };
};
