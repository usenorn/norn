import type { Instance } from "$lib/auth/instance";
import type { SignInFailure } from "$lib/auth/types";
import type { WorkspaceEntry } from "$lib/auth/workspace-sign-in";

export type SignInPreview = {
	auth?: Partial<Instance>;
	failure?: SignInFailure;
	entry?: WorkspaceEntry;
};

export const signInPreviewStates: Record<string, SignInPreview> = import.meta.env.DEV
	? {
			default: {},
			sso: { auth: { sso: { name: "Okta" } } },
			ssoonly: {
				auth: { password: false, sso: { name: "Okta" } },
			},
			invalid: { failure: { kind: "invalid_credentials", attemptsLeft: 3 } },
			locked: { failure: { kind: "account_locked", unlocksAt: "14:32" } },
			limited: { failure: { kind: "rate_limited" } },
			unavailable: { failure: { kind: "unavailable" } },
			misconfig: {
				auth: { sso: { name: "Okta" } },
				failure: {
					kind: "sso_unavailable",
					diagnostics: [
						{ key: "error", value: "sso_metadata_unreachable" },
						{ key: "provider", value: "Okta · SAML 2.0" },
						{ key: "url", value: "https://northwind.okta.com/app/exk4d/sso/saml/metadata" },
						{ key: "http", value: "404 · 312 ms" },
					],
				},
			},
			closed: { auth: { signupsOpen: false } },
			workspace_sso: {
				entry: {
					kind: "ready",
					signIn: {
						workspace: "northwind",
						name: "Northwind",
						password: true,
						sso: true,
						protocol: "oidc",
						host: "northwind.okta.com",
					},
				},
			},
			workspace_sso_only: {
				entry: {
					kind: "ready",
					signIn: {
						workspace: "northwind",
						name: "Northwind",
						password: false,
						sso: true,
						protocol: "oidc",
						host: "northwind.okta.com",
					},
				},
			},
			workspace_unknown: { entry: { kind: "unknown", workspace: "northwynd" } },
		}
	: {};
