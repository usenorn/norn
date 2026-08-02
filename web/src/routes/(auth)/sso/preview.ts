import type { AuthConfig, SsoExchange } from "$lib/auth/types";

export type SsoPreview = {
	auth?: Partial<AuthConfig>;
	exchange: SsoExchange;
};

export const ssoPreviewStates: Record<string, SsoPreview> = import.meta.env.DEV
	? {
			redirect: {
				auth: { sso: { name: "Okta" } },
				exchange: { status: "pending", phase: "redirecting" },
			},
			return: {
				auth: { sso: { name: "Okta" } },
				exchange: { status: "pending", phase: "returning" },
			},
			rejected: {
				auth: { sso: { name: "Okta" } },
				exchange: {
					status: "failed",
					failure: {
						kind: "rejected",
						reference: "Reference 4f2a-91c3 · 14:18:04 UTC",
						diagnostics: [
							{ key: "error", value: "access_denied" },
							{ key: "reason", value: "User is not assigned to this application" },
							{ key: "provider", value: "Okta · SAML 2.0" },
							{ key: "subject", value: "rae@northwind.co" },
							{ key: "time", value: "14:18:04 UTC" },
						],
					},
				},
			},
			nojit: {
				auth: { sso: { name: "Okta" } },
				exchange: {
					status: "failed",
					failure: {
						kind: "no_account",
						subject: "rae@northwind.co",
						reference: "Reference 4f2a-91c3 · 14:18:04 UTC",
						diagnostics: [
							{ key: "subject", value: "rae@northwind.co" },
							{ key: "provisioning", value: "just-in-time off" },
							{ key: "workspace", value: "northwind" },
							{ key: "time", value: "14:18:04 UTC" },
						],
					},
				},
			},
			down: {
				auth: { sso: { name: "Okta" } },
				exchange: {
					status: "failed",
					failure: {
						kind: "provider_unreachable",
						timeout: "10 seconds",
						reference: "Reference 4f2a-91c3 · 14:18:04 UTC",
						diagnostics: [
							{ key: "error", value: "provider_timeout" },
							{ key: "endpoint", value: "https://northwind.okta.com/app/exk4d/sso/saml" },
							{ key: "waited", value: "10.0s" },
							{ key: "time", value: "14:18:04 UTC" },
						],
					},
				},
			},
		}
	: {};
