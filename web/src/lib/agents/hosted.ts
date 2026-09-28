import type { components, operations } from "$lib/api/dashboard.gen";
import type { AiProviderFailureCode } from "$lib/ai-provider/ai-provider";

export type AgentTurn = components["schemas"]["AgentTurn"];
export type AgentConversationReply = components["schemas"]["AgentConversationReply"];
export type AgentConversationStop = components["schemas"]["AgentConversationStop"];

type ConverseResponses = operations["converseWithWorkspaceAgent"]["responses"];

export type ConversationProblem =
	ConverseResponses[403 | 404 | 409 | 422 | 503 | 500]["content"]["application/problem+json"];

export const conversationTurnLimit = 40;
export const conversationTurnLength = 8000;

export type ConversationExchange = { question: string; reply: AgentConversationReply };

export type ConversationFailure =
	| { kind: "not_configured" }
	| { kind: "provider"; code: AiProviderFailureCode }
	| { kind: "sealing_unavailable" }
	| { kind: "not_hosted" }
	| { kind: "disabled" }
	| { kind: "authority_missing" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type HostedConversation =
	| { kind: "idle"; exchanges: ConversationExchange[] }
	| { kind: "asking"; exchanges: ConversationExchange[]; question: string }
	| { kind: "failed"; exchanges: ConversationExchange[]; question: string; failure: ConversationFailure };

const providerFailureCodes: AiProviderFailureCode[] = [
	"key_rejected",
	"model_unavailable",
	"quota_exceeded",
	"rate_limited",
	"unreachable",
	"destination_refused",
];

export const transientFailures: AiProviderFailureCode[] = ["rate_limited", "unreachable"];

export function conversationTurns(exchanges: ConversationExchange[], question: string): AgentTurn[] {
	return [
		...exchanges.flatMap((exchange): AgentTurn[] => [
			{ role: "user", text: exchange.question },
			{ role: "assistant", text: spoken(exchange.reply) },
		]),
		{ role: "user", text: question },
	];
}

export function conversationFull(exchanges: ConversationExchange[]): boolean {
	return exchanges.length * 2 + 1 > conversationTurnLimit;
}

export function conversationFailure(
	problem: ConversationProblem | undefined,
	status: number
): ConversationFailure {
	if (status === 403) return { kind: "forbidden" };

	const code = problem && "code" in problem ? problem.code : undefined;

	switch (code) {
		case "ai_provider_not_configured":
			return { kind: "not_configured" };
		case "ai_provider_sealing_unavailable":
			return { kind: "sealing_unavailable" };
		case "agent_not_hosted":
			return { kind: "not_hosted" };
		case "agent_disabled":
			return { kind: "disabled" };
		case "agent_authority_missing":
			return { kind: "authority_missing" };
	}

	if (providerFailureCodes.includes(code as AiProviderFailureCode)) {
		return { kind: "provider", code: code as AiProviderFailureCode };
	}

	return { kind: "unavailable" };
}

const unfinishedNotes: Record<Exclude<AgentConversationStop, "answered">, string> = {
	round_limit: "The agent stopped after using as many tools as one answer may.",
	token_limit: "The agent stopped at this instance's limit on model usage for one answer.",
	time_limit: "The agent ran out of time before it finished.",
};

export function stopNote(stop: AgentConversationStop): string | null {
	return stop === "answered" ? null : unfinishedNotes[stop];
}

function spoken(reply: AgentConversationReply): string {
	return reply.text || stopNote(reply.stop) || "The agent gave no answer.";
}

const providerTitles: Record<AiProviderFailureCode, string> = {
	key_rejected: "The AI provider rejected the workspace's key",
	model_unavailable: "The workspace's key cannot use its default model",
	quota_exceeded: "The AI provider account is out of credit",
	rate_limited: "The AI provider is rate limiting the workspace's key",
	unreachable: "The AI provider could not be reached",
	destination_refused: "This instance will not connect to the AI provider's endpoint",
};

export function conversationFailureTitle(failure: ConversationFailure): string {
	switch (failure.kind) {
		case "not_configured":
			return "This workspace has no AI provider";
		case "provider":
			return providerTitles[failure.code];
		case "sealing_unavailable":
			return "The workspace's key cannot be read on this instance";
		case "not_hosted":
			return "This agent runs on a Norn Runner";
		case "disabled":
			return "This agent is disabled";
		case "authority_missing":
			return "This agent has no credential to act with";
		case "forbidden":
			return "You may not talk to this agent";
		case "unavailable":
			return "The agent did not answer";
	}
}

export function conversationFailureMessage(
	failure: ConversationFailure,
	administrator: boolean,
	selfHosted: boolean
): string {
	const settle = administrator
		? "Open the AI provider settings to fix it."
		: "Ask a workspace administrator to check the AI provider settings.";

	switch (failure.kind) {
		case "not_configured":
			return administrator
				? "Hosted agents answer with the workspace's own AI provider key. Add one to test this agent."
				: "Hosted agents answer with the workspace's own AI provider key. Ask a workspace administrator to add one.";
		case "provider":
			return failure.code === "rate_limited"
				? "Wait a minute and ask again."
				: failure.code === "unreachable"
					? "Nothing about the workspace's key changed. Ask again in a moment."
					: settle;
		case "sealing_unavailable":
			return selfHosted
				? "The server has no encryption key, so the stored AI provider key cannot be opened. Set NORN_SECURITY_ENCRYPTION_KEY on the server."
				: "Try again in a moment.";
		case "not_hosted":
			return "Choose Norn-hosted above to talk to it here.";
		case "disabled":
			return "Enable the agent before asking it anything.";
		case "authority_missing":
			return "Issue it a new credential, then ask again.";
		case "forbidden":
			return "Only the agent's owner and the workspace's administrators may test it.";
		case "unavailable":
			return "Nothing was changed. Ask again in a moment.";
	}
}

export function tokenCount(reply: AgentConversationReply): number {
	return reply.usage.inputTokens + reply.usage.outputTokens;
}
