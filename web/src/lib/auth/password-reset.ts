import type { operations } from "$lib/api/dashboard.gen";
import { minPasswordLength } from "./sign-up-schema";
import type { PasswordReset } from "./types";

type RequestResponses = operations["requestPasswordReset"]["responses"];
type ConfirmResponses = operations["confirmPasswordReset"]["responses"];

type CodedRequestProblem =
	RequestResponses[429 | 503]["content"]["application/problem+json"];
type CodedConfirmProblem =
	ConfirmResponses[400 | 409 | 429 | 503]["content"]["application/problem+json"];

export type ResetRequestAccepted = RequestResponses[202]["content"]["application/json"];

export type ResetRequestProblem =
	RequestResponses[422 | 429 | 500 | 503]["content"]["application/problem+json"];

export type ResetConfirmProblem =
	ConfirmResponses[400 | 409 | 422 | 429 | 500 | 503]["content"]["application/problem+json"];

function codedRequest(problem: ResetRequestProblem): problem is CodedRequestProblem {
	return "code" in problem;
}

function codedConfirm(problem: ResetConfirmProblem): problem is CodedConfirmProblem {
	return "code" in problem;
}

export function remaining(expiresAt: string, now: Date): string {
	const seconds = Math.max(0, Math.round((Date.parse(expiresAt) - now.getTime()) / 1000));
	const minutes = Math.floor(seconds / 60);

	return `${minutes}`.padStart(2, "0") + ":" + `${seconds % 60}`.padStart(2, "0");
}

export function resetSent(
	email: string,
	accepted: ResetRequestAccepted,
	now: Date
): PasswordReset {
	return { kind: "sent", email, expiresIn: remaining(accepted.expiresAt, now) };
}

export function resetRequestFailure(problem: ResetRequestProblem): PasswordReset | null {
	if (!codedRequest(problem)) return null;

	switch (problem.code) {
		case "mail_unavailable":
			return { kind: "mail_unavailable" };
		case "rate_limited":
			return { kind: "unavailable" };
	}
}

export function resetLinkFailure(problem: ResetConfirmProblem): PasswordReset | null {
	if (!codedConfirm(problem)) return null;

	switch (problem.code) {
		case "reset_link_expired":
			return { kind: "link_expired" };
		case "reset_link_used":
			return { kind: "link_used" };
		case "breach_check_unavailable":
		case "rate_limited":
			return { kind: "unavailable" };
	}
}

export function passwordMessage(code: string): string {
	switch (code) {
		case "breached":
			return "This password appears in a known breach. Choose a different one.";
		case "reused":
			return "You've used this password here before. Choose a different one.";
		case "too_short":
			return `Use at least ${minPasswordLength} characters.`;
		case "too_long":
			return "That password is too long.";
		case "required":
			return "Enter a password.";
		default:
			return "That password can't be used. Choose a different one.";
	}
}

export function emailMessage(code: string): string {
	switch (code) {
		case "required":
			return "Enter your email address.";
		case "malformed":
			return "Enter a valid email address.";
		case "personal_email":
			return "Use the address your team uses.";
		default:
			return "That address can't be used.";
	}
}
