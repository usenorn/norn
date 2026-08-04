import type { Instance } from "./instance";

export const authPreviewDeployments: Record<string, Partial<Instance>> = {
	cloud: { selfHosted: false, host: "norn.app", version: "" },
	"self-hosted": {
		selfHosted: true,
		host: "norn.northwind.internal",
		version: "1.4.2",
	},
};

export function deploymentPreview(url: URL): Partial<Instance> | undefined {
	if (!import.meta.env.DEV) return undefined;
	return authPreviewDeployments[url.searchParams.get("deployment") ?? ""];
}
