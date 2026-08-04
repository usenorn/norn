import { apiFor } from "$lib/api";
import type { SsoProviderConfiguration } from "$lib/workspace/sso";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({
	fetch,
	parent,
	url,
}): Promise<{ configuration: SsoProviderConfiguration }> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const params = { path: { workspaceId: workspace.id } };

	const { data: chosen, error } = await api.GET("/workspaces/{workspaceId}/sso", {
		fetch,
		params,
	});

	if (error) {
		if (error.status === 404) return { configuration: { kind: "unconfigured" } };

		return { configuration: { kind: error.status === 403 ? "forbidden" : "unavailable" } };
	}

	if (chosen?.protocol === "saml") {
		const { data } = await api.GET("/workspaces/{workspaceId}/sso/saml", { fetch, params });

		return data
			? { configuration: { kind: "saml", connection: data } }
			: { configuration: { kind: "unavailable" } };
	}

	const { data } = await api.GET("/workspaces/{workspaceId}/sso/oidc", { fetch, params });

	return data
		? { configuration: { kind: "oidc", connection: data } }
		: { configuration: { kind: "unavailable" } };
};
