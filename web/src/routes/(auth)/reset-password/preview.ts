import type { Instance } from "$lib/auth/instance";
import type { PasswordReset } from "$lib/auth/types";

export type ResetPasswordPreview = {
	auth?: Partial<Instance>;
	reset?: PasswordReset;
};

export const resetPasswordPreviewStates: Record<string, ResetPasswordPreview> = import.meta.env.DEV
	? {
			request: { reset: { kind: "request" } },
			sent: { reset: { kind: "sent", email: "rae@northwind.co", expiresIn: "59:41" } },
			form: { reset: { kind: "form" } },
			expired: { reset: { kind: "link_expired" } },
			used: { reset: { kind: "link_used" } },
			done: { reset: { kind: "changed" } },
			nomail: { reset: { kind: "mail_unavailable" } },
			unavailable: { reset: { kind: "unavailable" } },
			sso: {
				auth: { password: false, sso: { name: "Okta" } },
			},
		}
	: {};
