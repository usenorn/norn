import { keys } from "#lib/api/keys.js";
import type { MemoryDetail } from "#lib/memories/memories.js";
import { readMemory, readProjects } from "#lib/memories/memories.server.js";
import { memoryPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, params, parent, url }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.memory(params.memoryId));
	depends(keys.projects(workspace.id));

	if (import.meta.env.DEV && memoryPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { detail: { kind: "loading" } as MemoryDetail, projects: [] };
	}

	const [detail, projects] = await Promise.all([
		readMemory(locals.api, workspace.id, params.memoryId),
		readProjects(locals.api, workspace.id),
	]);

	return { detail, projects };
};
