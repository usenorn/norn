export type Diagnostic = { key: string; value: string };

export type SsoProvider = { name: string };

export type AuthConfig = {
	workspace: string;
	password: boolean;
	sso: SsoProvider | null;
	signupsOpen: boolean;
	selfHosted: boolean;
	host: string;
	instance: string | null;
};

export type SignInFailure =
	| { kind: "invalid_credentials"; attemptsLeft: number }
	| { kind: "account_locked"; unlocksAt: string }
	| { kind: "rate_limited" }
	| { kind: "unavailable" }
	| { kind: "sso_unavailable"; diagnostics: Diagnostic[] };

export type SsoPhase = "redirecting" | "returning";

export type SsoFailure =
	| { kind: "rejected"; diagnostics: Diagnostic[]; reference: string }
	| { kind: "no_account"; subject: string; diagnostics: Diagnostic[]; reference: string }
	| {
			kind: "provider_unreachable";
			timeout: string;
			diagnostics: Diagnostic[];
			reference: string;
	  };

export type SignUpOutcome =
	| { kind: "email_taken" }
	| { kind: "domain_uses_sso"; organization: string; provider: string }
	| { kind: "delivery_failed" }
	| {
			kind: "verification_sent";
			email: string;
			sentAt: string | null;
			expiresAt: string | null;
	  };

export type PasswordReset =
	| { kind: "request" }
	| { kind: "sent"; email: string; expiresIn: string }
	| { kind: "form" }
	| { kind: "link_expired" }
	| { kind: "link_used" }
	| { kind: "changed" }
	| { kind: "mail_unavailable" }
	| { kind: "unavailable" };

export type SsoExchange =
	| { status: "pending"; phase: SsoPhase }
	| { status: "failed"; failure: SsoFailure };
