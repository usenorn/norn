import { apiFor } from "$lib/api";
import type { Enforcement, SsoProviderConfiguration } from "$lib/workspace/sso";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({
	fetch,
	parent,
	url,
}): Promise<{ configuration: SsoProviderConfiguration; enforcement: Enforcement }> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const params = { path: { workspaceId: workspace.id } };

	const { data: chosen, error } = await api.GET("/workspaces/{workspaceId}/sso", {
		fetch,
		params,
	});

	const enforcement = await readEnforcement(api, fetch, workspace.id);

	if (error) {
		if (error.status === 404) return { configuration: { kind: "unconfigured" }, enforcement };

		return {
			configuration: { kind: error.status === 403 ? "forbidden" : "unavailable" },
			enforcement,
		};
	}

	if (chosen?.protocol === "saml") {
		const { data } = await api.GET("/workspaces/{workspaceId}/sso/saml", { fetch, params });

		return data
			? { configuration: { kind: "saml", connection: data }, enforcement }
			: { configuration: { kind: "unavailable" }, enforcement };
	}

	const { data } = await api.GET("/workspaces/{workspaceId}/sso/oidc", { fetch, params });

	return data
		? { configuration: { kind: "oidc", connection: data }, enforcement }
		: { configuration: { kind: "unavailable" }, enforcement };
};

async function readEnforcement(
	api: ReturnType<typeof apiFor>,
	fetch: typeof globalThis.fetch,
	workspaceId: string
): Promise<Enforcement> {
	const { data, error } = await api.GET("/workspaces/{workspaceId}/auth-policy", {
		fetch,
		params: { path: { workspaceId } },
	});

	if (error || !data) return { kind: "unavailable" };

	return { kind: "available", enforcing: data.enforcement === "sso" };
}
