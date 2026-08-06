import type { Client } from "openapi-fetch";
import { keys } from "$lib/api/keys";
import type { paths } from "$lib/api/dashboard.gen";
import type { Enforcement, SsoProviderConfiguration } from "$lib/workspace/sso";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<{ configuration: SsoProviderConfiguration; enforcement: Enforcement }> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();
	const params = { path: { workspaceId: workspace.id } };

	const { data: chosen, error } = await locals.api.GET("/workspaces/{workspaceId}/sso", {
		params,
	});

	const enforcement = await readEnforcement(locals.api, workspace.id);

	if (error) {
		if (error.status === 404) return { configuration: { kind: "unconfigured" }, enforcement };

		return {
			configuration: { kind: error.status === 403 ? "forbidden" : "unavailable" },
			enforcement,
		};
	}

	if (chosen?.protocol === "saml") {
		const { data } = await locals.api.GET("/workspaces/{workspaceId}/sso/saml", { params });

		return data
			? { configuration: { kind: "saml", connection: data }, enforcement }
			: { configuration: { kind: "unavailable" }, enforcement };
	}

	const { data } = await locals.api.GET("/workspaces/{workspaceId}/sso/oidc", { params });

	return data
		? { configuration: { kind: "oidc", connection: data }, enforcement }
		: { configuration: { kind: "unavailable" }, enforcement };
};

async function readEnforcement(api: Client<paths>, workspaceId: string): Promise<Enforcement> {
	const { data, error } = await api.GET("/workspaces/{workspaceId}/auth-policy", {
		params: { path: { workspaceId } },
	});

	if (error || !data) return { kind: "unavailable" };

	return { kind: "available", enforcing: data.enforcement === "sso" };
}
