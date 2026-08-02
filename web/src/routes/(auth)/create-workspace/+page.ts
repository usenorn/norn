import type { WorkspaceContext } from "$lib/workspace/types";
import type { PageLoad } from "./$types";

export const load: PageLoad = (): { workspace: WorkspaceContext } => ({
	workspace: { existingWorkspace: null },
});
