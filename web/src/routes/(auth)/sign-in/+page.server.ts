import { reachWorkspaceSignIn, type WorkspaceEntry } from "$lib/auth/workspace-sign-in";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	locals,
	url,
}): Promise<{ entry: WorkspaceEntry }> => ({
	entry: await reachWorkspaceSignIn(locals.api, url.searchParams.get("workspace") ?? ""),
});
