import { deploymentPreview } from "$lib/auth/preview";
import type { AuthConfig } from "$lib/auth/types";
import type { LayoutLoad } from "./$types";

const instanceAuth: AuthConfig = {
	workspace: "Norn",
	password: true,
	sso: null,
	signupsOpen: true,
	selfHosted: false,
	host: "norn.app",
	instance: null,
};

export const load: LayoutLoad = ({ url }): { auth: AuthConfig } => ({
	auth: { ...instanceAuth, ...deploymentPreview(url) },
});
