import type { AuthConfig, SignInFailure } from "$lib/auth/types";

export type SignInPreview = {
	auth?: Partial<AuthConfig>;
	failure?: SignInFailure;
};

export const signInPreviewStates: Record<string, SignInPreview> = import.meta.env.DEV
	? {
			default: {},
			sso: { auth: { sso: { name: "Okta" } } },
			ssoonly: {
				auth: { workspace: "Northwind", password: false, sso: { name: "Okta" } },
			},
			invalid: { failure: { kind: "invalid_credentials", attemptsLeft: 3 } },
			locked: { failure: { kind: "account_locked", unlocksAt: "14:32" } },
			limited: { failure: { kind: "rate_limited" } },
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
		}
	: {};
