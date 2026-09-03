import type { ProjectListing } from "$lib/projects/projects";
import { keys } from "$lib/api/keys";
import { projectsPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type ProjectsData = { listing: ProjectListing; team: { id: string; name: string } | null };

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

	const { teams } = await parent();
	const teamId = url.searchParams.get("teamId") ?? undefined;
	const team = teams?.find((candidate) => candidate.id === teamId) ?? null;

	if (import.meta.env.DEV && projectsPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" }, team };
	}

	const projects = await locals.api.GET("/workspaces/{workspaceId}/projects", {
		params: {
			path: { workspaceId: workspace.id },
			query: { archived: url.searchParams.get("archived") === "1", teamId },
		},
	});

	if (projects.error || !projects.data) return { listing: { kind: "unavailable" }, team };

	if (projects.data.length === 0) return { listing: { kind: "empty" }, team };

	return { listing: { kind: "ready", projects: projects.data }, team };
};
