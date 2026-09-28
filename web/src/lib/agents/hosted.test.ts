import { describe, expect, it } from "vitest";
import {
	conversationFailure,
	conversationFull,
	conversationTurnLimit,
	conversationTurns,
	type AgentConversationReply,
	type ConversationExchange,
	type ConversationProblem,
} from "./hosted";

function reply(overrides: Partial<AgentConversationReply> = {}): AgentConversationReply {
	return {
		text: "Three issues are in progress.",
		toolCalls: [],
		usage: { inputTokens: 100, outputTokens: 10 },
		stop: "answered",
		...overrides,
	};
}

function problem(status: number, code?: string): ConversationProblem {
	return { type: "about:blank", title: "Refused", status, code } as ConversationProblem;
}

describe("conversationTurns", () => {
	it("replays every exchange before the new question", () => {
		const exchanges: ConversationExchange[] = [{ question: "What is in progress?", reply: reply() }];

		expect(conversationTurns(exchanges, "Who owns them?")).toEqual([
			{ role: "user", text: "What is in progress?" },
			{ role: "assistant", text: "Three issues are in progress." },
			{ role: "user", text: "Who owns them?" },
		]);
	});

	it("never sends a blank answer back, because the server refuses one", () => {
		const cut = conversationTurns([{ question: "Look deeper.", reply: reply({ text: "", stop: "time_limit" }) }], "Try again?");

		expect(cut[1].text).toBe("The agent ran out of time before it finished.");
		expect(cut[1].text.trim()).not.toBe("");
	});
});

describe("conversationFull", () => {
	it("stops asking before the next question would pass the server's limit", () => {
		const exchange = { question: "Again?", reply: reply() };
		const most = Array.from({ length: (conversationTurnLimit - 1) / 2 }, () => exchange);

		expect(conversationFull(most.slice(0, -1))).toBe(false);
		expect(conversationFull(most)).toBe(false);
		expect(conversationFull([...most, exchange])).toBe(true);
	});
});

describe("conversationFailure", () => {
	it.each([
		[409, "ai_provider_not_configured", { kind: "not_configured" }],
		[409, "agent_not_hosted", { kind: "not_hosted" }],
		[409, "agent_disabled", { kind: "disabled" }],
		[409, "agent_authority_missing", { kind: "authority_missing" }],
		[422, "quota_exceeded", { kind: "provider", code: "quota_exceeded" }],
		[503, "ai_provider_sealing_unavailable", { kind: "sealing_unavailable" }],
		[403, undefined, { kind: "forbidden" }],
		[500, undefined, { kind: "unavailable" }],
	])("reads a %i %s as the failure a person can act on", (status, code, want) => {
		expect(conversationFailure(problem(status, code), status)).toEqual(want);
	});
});
