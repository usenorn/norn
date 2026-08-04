import { apiFor } from "$lib/api";
import type { SsoConfiguration } from "$lib/workspace/sso";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({
	fetch,
	parent,
	url,
}): Promise<{ configuration: SsoConfiguration }> => {
	const api = apiFor(url);

	const { workspace } = await parent();

	const { data, error } = await api.GET("/workspaces/{workspaceId}/sso/oidc", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	if (error) {
		if (error.status === 404) return { configuration: { kind: "unconfigured" } };

		return { configuration: { kind: error.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (!data) return { configuration: { kind: "unconfigured" } };

	return { configuration: { kind: "configured", connection: data } };
};
