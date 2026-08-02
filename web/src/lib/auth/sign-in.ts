import type { operations } from "$lib/api/dashboard.gen";
import type { SignInFailure } from "./types";

type SignInResponses = operations["signIn"]["responses"];

type CodedSignInProblem = SignInResponses[401 | 423 | 429]["content"]["application/problem+json"];

export type SignInProblem =
	SignInResponses[401 | 423 | 429 | 500]["content"]["application/problem+json"];

function coded(problem: SignInProblem): problem is CodedSignInProblem {
	return "code" in problem;
}

export function clockTime(instant: string): string {
	return new Date(instant).toLocaleTimeString(undefined, {
		hour: "2-digit",
		minute: "2-digit",
		hour12: false,
	});
}

export function signInFailure(problem: SignInProblem): SignInFailure | null {
	if (!coded(problem)) return null;

	switch (problem.code) {
		case "invalid_credentials":
			return { kind: "invalid_credentials", attemptsLeft: problem.attemptsLeft };
		case "account_locked":
			return { kind: "account_locked", unlocksAt: clockTime(problem.unlocksAt) };
		case "rate_limited":
			return { kind: "rate_limited" };
	}
}
