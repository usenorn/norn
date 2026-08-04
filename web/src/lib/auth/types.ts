export type Diagnostic = { key: string; value: string };

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
	| { kind: "verification_sent"; email: string; expiresAt: string }
	| { kind: "link_only"; email: string; expiresAt: string; url: string }
	| { kind: "email_taken" }
	| { kind: "domain_uses_sso"; organization: string; provider: string }
	| { kind: "closed" }
	| { kind: "rate_limited" }
	| { kind: "breach_check_unavailable" }
	| { kind: "unavailable" };

export type SignUpResend = "idle" | "limited" | "unavailable";

export type SignUpConfirmation =
	| { kind: "confirming" }
	| { kind: "confirmed"; email: string }
	| { kind: "no_token" }
	| { kind: "expired" }
	| { kind: "invalid" }
	| { kind: "used" }
	| { kind: "email_taken" }
	| { kind: "unavailable" };

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
