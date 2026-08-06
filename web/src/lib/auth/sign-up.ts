import type { operations } from "$lib/api/dashboard.gen";
import type { SignUpConfirmation, SignUpOutcome } from "./types";

type RequestResponses = operations["requestSignUp"]["responses"];
type ConfirmResponses = operations["confirmSignUp"]["responses"];

type CodedRequestProblem =
	RequestResponses[403 | 409 | 429 | 503]["content"]["application/problem+json"];
type CodedConfirmProblem = ConfirmResponses[400 | 409]["content"]["application/problem+json"];

export type SignUpRequested = RequestResponses[202]["content"]["application/json"];

export type SignUpProblem =
	RequestResponses[403 | 409 | 422 | 429 | 500 | 503]["content"]["application/problem+json"];

export type SignUpConfirmProblem =
	ConfirmResponses[400 | 409 | 500]["content"]["application/problem+json"];

function codedRequest(problem: SignUpProblem): problem is CodedRequestProblem {
	return "code" in problem;
}

function codedConfirm(problem: SignUpConfirmProblem): problem is CodedConfirmProblem {
	return "code" in problem;
}

export function signUpSent(requested: SignUpRequested): SignUpOutcome {
	const sent = {
		email: requested.email,
		requestedAt: requested.requestedAt,
		expiresAt: requested.expiresAt,
	};

	return requested.delivery === "link_only" && requested.url
		? { kind: "link_only", ...sent, url: requested.url }
		: { kind: "verification_sent", ...sent };
}

const personalEmailProviders = new Set([
	"aol",
	"gmail",
	"hotmail",
	"icloud",
	"live",
	"outlook",
	"proton",
	"yahoo",
]);

export function personalEmail(email: string): boolean {
	const at = email.lastIndexOf("@");

	if (at < 0) return false;

	const [label, ...rest] = email.slice(at + 1).toLowerCase().split(".");

	return rest.length > 0 && personalEmailProviders.has(label);
}

export function signUpFailure(problem: SignUpProblem): SignUpOutcome | null {
	if (!codedRequest(problem)) return null;

	switch (problem.code) {
		case "sign_up_closed":
			return { kind: "closed" };
		case "sign_up_used":
		case "sign_up_email_taken":
			return { kind: "email_taken" };
		case "rate_limited":
			return { kind: "rate_limited" };
		case "breach_check_unavailable":
			return { kind: "breach_check_unavailable" };
		case "sign_up_undeliverable":
			return { kind: "undeliverable" };
	}
}

export function signUpConfirmFailure(problem: SignUpConfirmProblem): SignUpConfirmation {
	if (!codedConfirm(problem)) return { kind: "unavailable" };

	switch (problem.code) {
		case "sign_up_expired":
			return { kind: "expired" };
		case "sign_up_invalid":
			return { kind: "invalid" };
		case "sign_up_used":
			return { kind: "used" };
		case "sign_up_email_taken":
			return { kind: "email_taken" };
	}
}

export function displayNameMessage(code: string): string {
	switch (code) {
		case "required":
			return "Enter your name.";
		case "too_long":
			return "That name is too long.";
		default:
			return "That name can't be used.";
	}
}
