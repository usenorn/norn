import { keys } from "$lib/api/keys";
import { docTree, type DocTree } from "$lib/docs/docs";
import { listDocs } from "$lib/docs/docs.server";
import type { Project } from "$lib/projects/projects";
import type { LayoutServerLoad } from "./$types";

export type DocsScope = { tree: DocTree; reachable: Project[] };

export const load: LayoutServerLoad = async ({ depends, locals, parent }): Promise<DocsScope> => {
	const { workspace } = await parent();

	depends(keys.docs(workspace.id));
	depends(keys.projects(workspace.id));

	const [listing, projects] = await Promise.all([
		listDocs(locals.api, workspace.id),
		locals.api.GET("/workspaces/{workspaceId}/projects", { params: { path: { workspaceId: workspace.id } } }),
	]);

	const reachable = (projects.data ?? []).filter((project) => !project.archived);

	if (!listing) return { tree: { kind: "unavailable" }, reachable };

	return { tree: docTree(listing.docs, projects.data ?? [], listing.partial), reachable };
};
