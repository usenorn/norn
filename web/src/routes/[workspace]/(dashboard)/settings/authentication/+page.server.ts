import { fail } from "@sveltejs/kit";
import type { Client } from "openapi-fetch";
import { message, setError, superValidate, type Infer } from "sveltekit-superforms";
import { zod4 } from "sveltekit-superforms/adapters";
import { keys } from "$lib/api/keys";
import type { paths } from "$lib/api/dashboard.gen";
import { samlConnectionSchema } from "$lib/workspace/saml-schema";
import { ssoConnectionSchema } from "$lib/workspace/sso-schema";
import {
	failureMessage,
	parseScopes,
	saveFailure,
	type Enforcement,
	type SsoOutcome,
	type SsoProviderConfiguration,
} from "$lib/workspace/sso";
import type { Actions, PageServerLoad } from "./$types";

type OidcForm = Infer<typeof ssoConnectionSchema>;
type SamlForm = Infer<typeof samlConnectionSchema>;

const oidcFormId = "sso-connection-form";
const samlFormId = "saml-connection-form";

const unreachable: SsoOutcome = { kind: "failed", failure: { kind: "unavailable" } };

export const load: PageServerLoad = async ({ depends, route, locals, parent }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	return {
		configuration: await readConfiguration(locals.api, workspace.id),
		enforcement: await readEnforcement(locals.api, workspace.id),
		oidcForm: await superValidate<OidcForm, SsoOutcome>(zod4(ssoConnectionSchema), {
			id: oidcFormId,
		}),
		samlForm: await superValidate<SamlForm, SsoOutcome>(zod4(samlConnectionSchema), {
			id: samlFormId,
		}),
	};
};

async function readConfiguration(
	api: Client<paths>,
	workspaceId: string
): Promise<SsoProviderConfiguration> {
	const params = { path: { workspaceId } };

	const { data: chosen, error } = await api.GET("/workspaces/{workspaceId}/sso", { params });

	if (error) {
		if (error.status === 404) return { kind: "unconfigured" };

		return { kind: error.status === 403 ? "forbidden" : "unavailable" };
	}

	if (chosen?.protocol === "saml") {
		const { data } = await api.GET("/workspaces/{workspaceId}/sso/saml", { params });

		return data ? { kind: "saml", connection: data } : { kind: "unavailable" };
	}

	const { data } = await api.GET("/workspaces/{workspaceId}/sso/oidc", { params });

	return data ? { kind: "oidc", connection: data } : { kind: "unavailable" };
}

async function readEnforcement(api: Client<paths>, workspaceId: string): Promise<Enforcement> {
	const { data, error } = await api.GET("/workspaces/{workspaceId}/auth-policy", {
		params: { path: { workspaceId } },
	});

	if (error || !data) return { kind: "unavailable" };

	return { kind: "available", enforcing: data.enforcement === "sso" };
}

async function workspaceIdFor(api: Client<paths>, slug: string): Promise<string | null> {
	const { data } = await api.GET("/workspaces");

	return data?.find((candidate) => candidate.slug === slug)?.id ?? null;
}

export const actions: Actions = {
	oidc: async ({ locals, params, request }) => {
		const form = await superValidate<OidcForm, SsoOutcome>(request, zod4(ssoConnectionSchema), {
			id: oidcFormId,
		});

		const clientSecret = form.data.clientSecret;

		form.data.clientSecret = "";

		if (!form.valid) return fail(400, { form });

		const workspaceId = await workspaceIdFor(locals.api, params.workspace);

		if (!workspaceId) return message(form, unreachable, { status: 500 });

		const { data, error } = await locals.api.PUT("/workspaces/{workspaceId}/sso/oidc", {
			params: { path: { workspaceId } },
			body: {
				issuer: form.data.issuer,
				endpoints: form.data.manual
					? {
							issuer: form.data.issuer,
							authorizationEndpoint: form.data.authorizationEndpoint,
							tokenEndpoint: form.data.tokenEndpoint,
							jwksUri: form.data.jwksUri,
							userinfoEndpoint: form.data.userinfoEndpoint || undefined,
						}
					: undefined,
				clientId: form.data.clientId,
				clientSecret: clientSecret || undefined,
				scopes: parseScopes(form.data.scopes),
				groupsClaim: form.data.groupsClaim || undefined,
				provisioning: form.data.provisioning,
			},
		});

		if (error) {
			const failure = saveFailure(error);

			if (failure.kind === "stage" && failure.stage === "discovery") {
				setError(form, "issuer", failureMessage(failure));
			}

			return message(form, { kind: "failed", failure }, { status: 400 });
		}

		if (!data) return message(form, unreachable, { status: 500 });

		return message(form, { kind: "saved" });
	},

	saml: async ({ locals, params, request }) => {
		const form = await superValidate<SamlForm, SsoOutcome>(request, zod4(samlConnectionSchema), {
			id: samlFormId,
		});

		if (!form.valid) return fail(400, { form });

		const workspaceId = await workspaceIdFor(locals.api, params.workspace);

		if (!workspaceId) return message(form, unreachable, { status: 500 });

		const { data, error } = await locals.api.PUT("/workspaces/{workspaceId}/sso/saml", {
			params: { path: { workspaceId } },
			body: {
				metadataUrl: form.data.source === "url" ? form.data.metadataUrl : undefined,
				metadata: form.data.source === "paste" ? form.data.metadata : undefined,
				descriptor:
					form.data.source === "manual"
						? {
								entityId: form.data.entityId,
								ssoUrl: form.data.ssoUrl,
								certificates: [form.data.certificate],
								expiresAt: new Date().toISOString(),
							}
						: undefined,
				allowIdpInitiated: form.data.allowIdpInitiated,
				provisioning: form.data.provisioning,
				mapping: {
					email: form.data.emailAttribute || undefined,
					name: form.data.nameAttribute || undefined,
					groups: form.data.groupsAttribute || undefined,
				},
			},
		});

		if (error) {
			return message(form, { kind: "failed", failure: saveFailure(error) }, { status: 400 });
		}

		if (!data) return message(form, unreachable, { status: 500 });

		return message(form, { kind: "saved" });
	},
};
