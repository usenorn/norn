import { apiFor } from "$lib/api";
import type { ProjectListing } from "$lib/projects/projects";
import { projectsPreviewStates } from "./preview";
import type { PageLoad } from "./$types";

export type ProjectsData = { listing: ProjectListing };

export const load: PageLoad = async ({ fetch, parent, url }): Promise<ProjectsData> => {
	const api = apiFor(url);

	const { workspace } = await parent();

	if (import.meta.env.DEV && projectsPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { listing: { kind: "loading" } };
	}

	const projects = await api.GET("/workspaces/{workspaceId}/projects", {
		fetch,
		params: {
			path: { workspaceId: workspace.id },
			query: { archived: url.searchParams.get("archived") === "1" },
		},
	});

	if (projects.error || !projects.data) return { listing: { kind: "unavailable" } };

	if (projects.data.length === 0) return { listing: { kind: "empty" } };

	return { listing: { kind: "ready", projects: projects.data } };
};
