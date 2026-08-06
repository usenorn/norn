import type { Instance } from "$lib/auth/instance";
import type { SignUpOutcome, SignUpResend } from "$lib/auth/types";
import type { SignUpInput } from "$lib/auth/sign-up-schema";

export type SignUpPreview = {
	auth?: Partial<Instance>;
	form?: Partial<SignUpInput>;
	outcome?: SignUpOutcome;
	resend?: SignUpResend;
	busy?: boolean;
};

export const signUpPreviewStates: Record<string, SignUpPreview> = import.meta.env.DEV
	? {
			default: { auth: { sso: { name: "Okta" }, breachCheck: true } },
			no_breach_check: { auth: { breachCheck: false } },
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
			blocked: {
				form: {
					name: "Rae Okafor",
					email: "rae@gmail.com",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
			},
			failed: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: { kind: "undeliverable" },
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
					requestedAt: "2026-08-05T14:22:00Z",
					expiresAt: "2026-08-05T15:22:00Z",
				},
			},
			verify_sending: {
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
					requestedAt: "2026-08-05T14:22:00Z",
					expiresAt: "2026-08-05T15:22:00Z",
				},
				busy: true,
			},
			resend_limited: {
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
					requestedAt: "2026-08-05T14:22:00Z",
					expiresAt: "2026-08-05T15:22:00Z",
				},
				resend: "limited",
			},
			resend_unavailable: {
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
					requestedAt: "2026-08-05T14:22:00Z",
					expiresAt: "2026-08-05T15:22:00Z",
				},
				resend: "unavailable",
			},
			linkonly: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: {
					kind: "link_only",
					email: "rae@northwind.co",
					requestedAt: "2026-08-05T14:22:00Z",
					expiresAt: "2026-08-05T15:22:00Z",
					url: "https://norn.northwind.internal/sign-up/confirm?token=zk4p9w2m",
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
			sso: {
				auth: { sso: { name: "Okta" } },
				outcome: { kind: "domain_uses_sso", organization: "Northwind", provider: "Okta" },
			},
			ssodomain: {
				auth: { sso: { name: "Okta" } },
				outcome: { kind: "domain_uses_sso", organization: "Northwind", provider: "Okta" },
			},
			closed: { auth: { signupsOpen: false } },
			limited: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: { kind: "rate_limited" },
			},
			breach: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: { kind: "breach_check_unavailable" },
			},
			unavailable: {
				form: {
					name: "Rae Okafor",
					email: "rae@northwind.co",
					password: "northwind-cycle-24",
					passwordConfirm: "northwind-cycle-24",
					terms: true,
				},
				outcome: { kind: "unavailable" },
			},
		}
	: {};
