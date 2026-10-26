import type { Instance } from "#lib/auth/instance.js";
import type { SignInOutcome } from "#lib/auth/sign-in-step.js";
import type { WorkspaceEntry } from "#lib/auth/workspace-sign-in.js";

export type SignInPreview = {
	auth?: Partial<Instance>;
	outcome?: SignInOutcome;
	entry?: WorkspaceEntry;
};

export const signInPreviewStates: Record<string, SignInPreview> = import.meta.env.DEV
	? {
			default: {},
			identify_limited: { outcome: { step: { kind: "identify" }, failure: { kind: "rate_limited" } } },
			identify_unavailable: { outcome: { step: { kind: "identify" }, failure: { kind: "unavailable" } } },
			password: {
				outcome: {
					step: {
						kind: "methods",
						email: "rae@northwind.co",
						workspace: "",
						options: [{ method: "password" }],
					},
				},
			},
			choose: {
				outcome: {
					step: {
						kind: "methods",
						email: "rae@northwind.co",
						workspace: "",
						options: [
							{
								method: "sso",
								workspace: {
									workspace: "northwind",
									name: "Northwind",
									protocol: "oidc",
									host: "northwind.okta.com",
								},
							},
							{
								method: "sso",
								workspace: {
									workspace: "northwind-labs",
									name: "Northwind Labs",
									protocol: "saml",
									host: "idp.northwind-labs.co",
								},
							},
						],
					},
				},
			},
			choose_with_password: {
				outcome: {
					step: {
						kind: "methods",
						email: "rae@northwind.co",
						workspace: "",
						options: [
							{
								method: "sso",
								workspace: {
									workspace: "northwind",
									name: "Northwind",
									protocol: "oidc",
									host: "northwind.okta.com",
								},
							},
							{
								method: "sso",
								workspace: {
									workspace: "northwind-labs",
									name: "Northwind Labs",
									protocol: "saml",
									host: "idp.northwind-labs.co",
								},
							},
							{ method: "password" },
						],
					},
				},
			},
			closed_address: {
				auth: { password: false },
				outcome: { step: { kind: "closed", email: "rae@gmail.com" } },
			},
			invalid: {
				outcome: {
					step: { kind: "methods", email: "rae@northwind.co", workspace: "", options: [{ method: "password" }] },
					failure: { kind: "invalid_credentials", attemptsLeft: 3 },
				},
			},
			locked: {
				outcome: {
					step: { kind: "methods", email: "rae@northwind.co", workspace: "", options: [{ method: "password" }] },
					failure: { kind: "account_locked", unlocksAt: "2026-08-06T14:32:00Z" },
				},
			},
			limited: {
				outcome: {
					step: { kind: "methods", email: "rae@northwind.co", workspace: "", options: [{ method: "password" }] },
					failure: { kind: "rate_limited" },
				},
			},
			unavailable: {
				outcome: {
					step: { kind: "methods", email: "rae@northwind.co", workspace: "", options: [{ method: "password" }] },
					failure: { kind: "unavailable" },
				},
			},
			misconfig: {
				auth: { sso: { name: "Okta" } },
				outcome: {
					step: { kind: "methods", email: "rae@northwind.co", workspace: "", options: [{ method: "password" }] },
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
			},
			closed: { auth: { signupsOpen: false } },
			workspace_sso: {
				entry: {
					kind: "ready",
					signIn: {
						workspace: "northwind",
						name: "Northwind",
						preferred: "sso",
						options: [
							{
								method: "sso",
								workspace: {
									workspace: "northwind",
									name: "Northwind",
									protocol: "oidc",
									host: "northwind.okta.com",
								},
							},
							{ method: "password" },
						],
					},
				},
				outcome: {
					step: {
						kind: "methods",
						email: "",
						workspace: "northwind",
						options: [
							{
								method: "sso",
								workspace: {
									workspace: "northwind",
									name: "Northwind",
									protocol: "oidc",
									host: "northwind.okta.com",
								},
							},
							{ method: "password" },
						],
					},
				},
			},
			workspace_sso_only: {
				entry: {
					kind: "ready",
					signIn: {
						workspace: "northwind",
						name: "Northwind",
						preferred: "sso",
						options: [
							{
								method: "sso",
								workspace: {
									workspace: "northwind",
									name: "Northwind",
									protocol: "oidc",
									host: "northwind.okta.com",
								},
							},
						],
					},
				},
				outcome: {
					step: {
						kind: "methods",
						email: "",
						workspace: "northwind",
						options: [
							{
								method: "sso",
								workspace: {
									workspace: "northwind",
									name: "Northwind",
									protocol: "oidc",
									host: "northwind.okta.com",
								},
							},
						],
					},
				},
			},
			workspace_unknown: { entry: { kind: "unknown", workspace: "northwynd" } },
		}
	: {};
