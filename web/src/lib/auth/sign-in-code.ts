import type { operations } from "$lib/api/dashboard.gen";
import { authPath } from "./return-to";
import type { SignInCodeFailure } from "./types";

type VerifyResponses = operations["verifySignInCode"]["responses"];

type CodedVerifyProblem = VerifyResponses[401 | 429]["content"]["application/problem+json"];

export type VerifySignInCodeProblem =
	VerifyResponses[401 | 409 | 429 | 500]["content"]["application/problem+json"];

function coded(problem: VerifySignInCodeProblem): problem is CodedVerifyProblem {
	return "code" in problem;
}

export function signInCodeFailure(problem: VerifySignInCodeProblem): SignInCodeFailure {
	if (!coded(problem)) {
		return problem.status === 409 ? { kind: "spent" } : { kind: "unavailable" };
	}

	switch (problem.code) {
		case "sign_in_code_incorrect":
			return { kind: "incorrect", attemptsLeft: problem.attemptsLeft };
		case "rate_limited":
			return { kind: "rate_limited" };
	}
}

export function codeEntry(challengeId: string, url: URL): string {
	return authPath(url, `/sign-in/code?challenge=${encodeURIComponent(challengeId)}`);
}
