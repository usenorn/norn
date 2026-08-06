import type { WorkspaceContext } from "$lib/workspace/types";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	locals,
}): Promise<{ workspace: WorkspaceContext }> => {
	const { data } = await locals.api.GET("/workspaces");

	return { workspace: { existingWorkspace: data?.[0]?.name ?? null } };
};
