import type { SignUpConfirmation } from "$lib/auth/types";

export const signUpConfirmPreviewStates: Record<string, SignUpConfirmation> = import.meta.env.DEV
	? {
			confirming: { kind: "confirming" },
			confirmed: { kind: "confirmed", email: "rae@northwind.co" },
			no_token: { kind: "no_token" },
			expired: { kind: "expired" },
			invalid: { kind: "invalid" },
			used: { kind: "used" },
			email_taken: { kind: "email_taken" },
			unavailable: { kind: "unavailable" },
		}
	: {};
