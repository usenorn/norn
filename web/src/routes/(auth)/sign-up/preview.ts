import type { AuthConfig, SignUpOutcome } from "$lib/auth/types";
import type { SignUpInput } from "$lib/auth/sign-up-schema";

export type SignUpPreview = {
	auth?: Partial<AuthConfig>;
	form?: Partial<SignUpInput>;
	outcome?: SignUpOutcome;
	busy?: boolean;
};

export const signUpPreviewStates: Record<string, SignUpPreview> = import.meta.env.DEV
	? {
			default: { auth: { sso: { name: "Okta" } } },
			weak: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "cycle24",
					passwordConfirm: "cycle24",
					terms: true,
				},
			},
			mismatch: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-2",
					terms: true,
				},
			},
			taken: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: { kind: "email_taken" },
			},
			blocked: {
				form: {
					name: "Rae Okafor",
					email: "rae@gmail.com",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
			},
			creating: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				busy: true,
			},
			verify: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: {
					kind: "verification_sent",
					email: "rae@northwind.co",
					sentAt: "14:22 UTC",
					expiresAt: "15:22 UTC · one use",
				},
			},
			sso: {
				auth: { sso: { name: "Okta" } },
				outcome: { kind: "domain_uses_sso", organization: "Northwind", provider: "Okta" },
			},
			closed: { auth: { signupsOpen: false } },
			failed: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: { kind: "delivery_failed" },
			},
		}
	: {};
