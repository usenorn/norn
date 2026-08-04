import { apiFor } from "$lib/api";
import type { WorkspaceContext } from "$lib/workspace/types";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, url}): Promise<{ workspace: WorkspaceContext }> => {
	const api = apiFor(url);

	const { data } = await api.GET("/workspaces", { fetch });

	return { workspace: { existingWorkspace: data?.[0]?.name ?? null } };
};
