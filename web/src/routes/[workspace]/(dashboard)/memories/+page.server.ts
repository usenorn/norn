import { keys } from "#lib/api/keys.js";
import { memoryFilterOf, type MemoryList } from "#lib/memories/memories.js";
import { listMemories, readProjects } from "#lib/memories/memories.server.js";
import { memoriesPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, locals, parent, url }) => {
	const { workspace } = await parent();

	depends(keys.memories(workspace.id));
	depends(keys.projects(workspace.id));

	const filter = memoryFilterOf(url.searchParams.get("filter"));

	if (import.meta.env.DEV && memoriesPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { filter, list: { kind: "loading" } as MemoryList, projects: [] };
	}

	const [list, projects] = await Promise.all([
		listMemories(locals.api, workspace.id, filter, url.searchParams.get("cursor") ?? undefined),
		readProjects(locals.api, workspace.id),
	]);

	return { filter, list, projects };
};
