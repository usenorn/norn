import type { AuthConfig } from "./types";

export const authPreviewDeployments: Record<string, Partial<AuthConfig>> = {
	cloud: { selfHosted: false, host: "norn.app", instance: null },
	"self-hosted": {
		selfHosted: true,
		host: "norn.northwind.internal",
		instance: "norn.northwind.internal · v1.4.2",
	},
};

export function deploymentPreview(url: URL): Partial<AuthConfig> | undefined {
	if (!import.meta.env.DEV) return undefined;
	return authPreviewDeployments[url.searchParams.get("deployment") ?? ""];
}
