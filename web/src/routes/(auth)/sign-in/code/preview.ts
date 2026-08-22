import type { SignInCodeFailure } from "$lib/auth/types";

export type SignInCodePreview = {
	failure?: SignInCodeFailure;
	email?: string;
};

export const signInCodePreviewStates: Record<string, SignInCodePreview> = import.meta.env.DEV
	? {
			default: {},
			sent: { email: "rae@northwind.co" },
			incorrect: { email: "rae@northwind.co", failure: { kind: "incorrect", attemptsLeft: 4 } },
			last_attempt: {
				email: "rae@northwind.co",
				failure: { kind: "incorrect", attemptsLeft: 1 },
			},
			spent: { email: "rae@northwind.co", failure: { kind: "spent" } },
			limited: { email: "rae@northwind.co", failure: { kind: "rate_limited" } },
			unavailable: { email: "rae@northwind.co", failure: { kind: "unavailable" } },
		}
	: {};
