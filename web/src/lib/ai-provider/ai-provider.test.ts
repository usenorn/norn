import { describe, expect, it } from "vitest";
import { aiProviderFailure, failureMessage, type AiProviderProblem } from "./ai-provider";

function problem(fields: Partial<AiProviderProblem> & { status: number }): AiProviderProblem {
	return { type: "about:blank", title: "Problem", ...fields } as AiProviderProblem;
}

describe("reading what the provider said", () => {
	it("keeps the provider's verdict and its own words, so the screen can say which fix applies", () => {
		expect(
			aiProviderFailure(problem({ status: 422, code: "key_rejected", detail: "Incorrect API key provided" }))
		).toEqual({ kind: "provider", code: "key_rejected", detail: "Incorrect API key provided" });
	});

	it("reads a refusal to encrypt as an instance problem, not a bad key", () => {
		expect(aiProviderFailure(problem({ status: 503, code: "ai_provider_sealing_unavailable" }))).toEqual({
			kind: "sealing_unavailable",
		});
	});

	it("reads a 403 as forbidden even though the body names its own code", () => {
		expect(aiProviderFailure(problem({ status: 403, code: "forbidden" } as never))).toEqual({ kind: "forbidden" });
	});

	it("falls back to unavailable for anything it does not recognise", () => {
		expect(aiProviderFailure(problem({ status: 500 }))).toEqual({ kind: "unavailable" });
	});
});

describe("telling an administrator what to do", () => {
	it("names the server setting only where the reader runs the server", () => {
		const sealing = { kind: "sealing_unavailable" } as const;

		expect(failureMessage(sealing, "openai", true)).toContain("NORN_SECURITY_ENCRYPTION_KEY");
		expect(failureMessage(sealing, "openai", false)).not.toContain("NORN_SECURITY_ENCRYPTION_KEY");
	});
});
