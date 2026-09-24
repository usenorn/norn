import type { components, operations } from "$lib/api/dashboard.gen";

export type AiProvider = components["schemas"]["WorkspaceAiProvider"];
export type AiProviderKind = components["schemas"]["AiProviderKind"];
export type AiProviderFailureCode = components["schemas"]["AiProviderFailure"];

type ProblemOf<Responses> = Responses[keyof Responses] extends infer Response
	? Response extends { content: { "application/problem+json": infer Problem } }
		? Problem
		: never
	: never;

export type AiProviderProblem =
	| ProblemOf<operations["setWorkspaceAiProvider"]["responses"]>
	| ProblemOf<operations["testWorkspaceAiProvider"]["responses"]>
	| ProblemOf<operations["listWorkspaceAiProviderModels"]["responses"]>
	| ProblemOf<operations["removeWorkspaceAiProvider"]["responses"]>;

export type AiProviderFailure =
	| { kind: "provider"; code: AiProviderFailureCode; detail?: string }
	| { kind: "sealing_unavailable" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type AiProviderModels =
	| { kind: "listed"; models: string[] }
	| { kind: "failed"; failure: AiProviderFailure };

export type AiProviderView =
	| { kind: "loading" }
	| { kind: "unconfigured" }
	| { kind: "configured"; provider: AiProvider; models: AiProviderModels }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type AiProviderOutcome =
	| { kind: "idle" }
	| { kind: "saved" }
	| { kind: "tested" }
	| { kind: "removed" }
	| { kind: "failed"; failure: AiProviderFailure };

export const aiProviderKinds: AiProviderKind[] = ["openai"];

const aiProviderFailureCodes: AiProviderFailureCode[] = [
	"key_rejected",
	"model_unavailable",
	"quota_exceeded",
	"rate_limited",
	"unreachable",
	"destination_refused",
];

function isFailureCode(code: unknown): code is AiProviderFailureCode {
	return aiProviderFailureCodes.includes(code as AiProviderFailureCode);
}

export const providerLabels: Record<AiProviderKind, string> = {
	openai: "OpenAI",
};

export function aiProviderFailure(problem: AiProviderProblem): AiProviderFailure {
	if (problem.status === 403) return { kind: "forbidden" };

	if ("code" in problem && problem.code === "ai_provider_sealing_unavailable") {
		return { kind: "sealing_unavailable" };
	}

	if ("code" in problem && isFailureCode(problem.code)) {
		return { kind: "provider", code: problem.code, detail: problem.detail };
	}

	return { kind: "unavailable" };
}

const providerTitles: Record<AiProviderFailureCode, (label: string) => string> = {
	key_rejected: (label) => `${label} rejected this key`,
	model_unavailable: () => "This key cannot use that model",
	quota_exceeded: (label) => `The ${label} account is out of credit`,
	rate_limited: (label) => `${label} is rate limiting this key`,
	unreachable: (label) => `${label} could not be reached`,
	destination_refused: () => "This instance will not connect to that endpoint",
};

const providerAdvice: Record<AiProviderFailureCode, (label: string, selfHosted: boolean) => string> = {
	key_rejected: () =>
		"Check that the whole key was copied and that it has not been revoked. A restricted key needs permission to list models and to make requests.",
	model_unavailable: () =>
		"The model may have been retired, or the key's project may not have access to it. Choose another model.",
	quota_exceeded: (label) =>
		`Add credit or raise the usage limit on the ${label} account this key belongs to, then test again.`,
	rate_limited: () => "Wait a minute and test again.",
	unreachable: (label, selfHosted) =>
		selfHosted
			? `Check that this server can make outbound requests to ${label}, or to the endpoint you set. Nothing about the key changed.`
			: "Nothing about the key changed. Try again in a moment.",
	destination_refused: (_label, selfHosted) =>
		selfHosted
			? "The endpoint resolves to an address Norn refuses by default. For a server on your own network, allow a private network address. Loopback and link-local addresses also need NORN_OPENAI_ALLOWED_DESTINATIONS on the server."
			: "The endpoint resolves to a private or internal address. Norn cloud only connects to public endpoints.",
};

export function failureTitle(failure: AiProviderFailure, provider: AiProviderKind): string {
	switch (failure.kind) {
		case "provider":
			return providerTitles[failure.code](providerLabels[provider]);
		case "sealing_unavailable":
			return "Keys cannot be stored on this instance";
		case "forbidden":
			return "You cannot change this";
		case "unavailable":
			return "Something went wrong";
	}
}

export function failureMessage(
	failure: AiProviderFailure,
	provider: AiProviderKind,
	selfHosted: boolean
): string {
	switch (failure.kind) {
		case "provider":
			return providerAdvice[failure.code](providerLabels[provider], selfHosted);
		case "sealing_unavailable":
			return selfHosted
				? "Set NORN_SECURITY_ENCRYPTION_KEY on the server to 32 random bytes, base64 encoded, and restart it. Norn stores no API key it cannot encrypt."
				: "Norn cannot encrypt keys right now, so none can be saved or used. Try again later.";
		case "forbidden":
			return "Only a workspace administrator can configure the AI provider.";
		case "unavailable":
			return "We could not reach the server. Nothing changed.";
	}
}
