import { reachInstance, type Instance } from "$lib/auth/instance";
import { deploymentPreview } from "$lib/auth/preview";
import type { LayoutLoad } from "./$types";

export const load: LayoutLoad = async ({ fetch, url }): Promise<{ auth: Instance }> => ({
	auth: { ...(await reachInstance(fetch, url)), ...deploymentPreview(url) },
});
