import { keys } from "#lib/api/keys.js";
import type { MemoryHistory } from "#lib/memories/memories.js";
import { readMemoryHistory, readProjects } from "#lib/memories/memories.server.js";
import { memoryHistoryPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, locals, params, parent, url }) => {
	const { workspace } = await parent();

	depends(keys.memory(params.memoryId));

	if (import.meta.env.DEV && memoryHistoryPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { history: { kind: "loading" } as MemoryHistory, projects: [] };
	}

	const [history, projects] = await Promise.all([
		readMemoryHistory(
			locals.api,
			workspace.id,
			params.memoryId,
			url.searchParams.get("revision") ?? undefined,
			url.searchParams.get("cursor") ?? undefined
		),
		readProjects(locals.api, workspace.id),
	]);

	return { history, projects };
};
