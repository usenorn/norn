import { reachInstance, type Instance } from "$lib/auth/instance";
import { deploymentPreview } from "$lib/auth/preview";
import type { LayoutServerLoad } from "./$types";

export const load: LayoutServerLoad = async ({ locals, url }): Promise<{ auth: Instance }> => ({
	auth: { ...(await reachInstance(locals.api, url)), ...deploymentPreview(url) },
});
