import { reachWorkspaceSignIn, type WorkspaceEntry } from "$lib/auth/workspace-sign-in";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, url }): Promise<{ entry: WorkspaceEntry }> => ({
	entry: await reachWorkspaceSignIn(fetch, url, url.searchParams.get("workspace") ?? ""),
});
