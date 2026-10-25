import { reachInstance, type Instance } from "#lib/auth/instance.js";
import { deploymentPreview } from "#lib/auth/preview.js";
import type { LayoutServerLoad } from "./$types";

export const load: LayoutServerLoad = async ({ locals, url }): Promise<{ auth: Instance }> => ({
	auth: { ...(await reachInstance(locals.api, url)), ...deploymentPreview(url) },
});
