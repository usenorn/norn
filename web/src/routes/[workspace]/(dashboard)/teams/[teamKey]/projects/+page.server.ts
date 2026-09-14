import { error } from "@sveltejs/kit";
import type { ProjectListing } from "$lib/projects/projects";
import { keys } from "$lib/api/keys";
import { teamProjectsPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type TeamProjectsData = {
	listing: ProjectListing;
	team: { id: string; name: string };
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<TeamProjectsData> => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();

	depends(keys.projects(workspace.id));

	const team = (teams ?? []).find((candidate) => candidate.key === params.teamKey.toUpperCase());
	const previewing =
		import.meta.env.DEV &&
		Boolean(teamProjectsPreviewStates[url.searchParams.get("state") ?? ""]);

	if (!team) {
		if (previewing) {
			return { listing: { kind: "loading" }, team: { id: "", name: params.teamKey } };
		}

		error(404, "That team does not exist, or you are not on it.");
	}

	const named = { id: team.id, name: team.name };

	if (previewing) return { listing: { kind: "loading" }, team: named };

	const archived = url.searchParams.get("archived") === "1";

	const projects = await locals.api.GET("/workspaces/{workspaceId}/projects", {
		params: { path: { workspaceId: workspace.id }, query: { archived, teamId: team.id } },
	});

	if (projects.error || !projects.data) return { listing: { kind: "unavailable" }, team: named };

	if (projects.data.length === 0) return { listing: { kind: "no_matches" }, team: named };

	return { listing: { kind: "ready", projects: projects.data }, team: named };
};
