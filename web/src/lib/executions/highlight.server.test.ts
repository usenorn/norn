import { describe, expect, it } from "vitest";
import { languageOf, parseDiff } from "./diff";
import { highlighted } from "./highlight.server";

const patch = [
	"diff --git a/greeting/greeting.go b/greeting/greeting.go",
	"--- a/greeting/greeting.go",
	"+++ b/greeting/greeting.go",
	"@@ -1,3 +1,4 @@",
	" package greeting",
	"-// Greet says hello",
	"+// Greet says hello to someone",
	'+func Farewell(name string) string { return "See you, " + name }',
	" ",
	"diff --git a/notes.unknown b/notes.unknown",
	"--- a/notes.unknown",
	"+++ b/notes.unknown",
	"@@ -1 +1 @@",
	"-old",
	"+new",
].join("\n");

describe("highlighting a diff", () => {
	it("guesses the language from the file name", () => {
		expect(languageOf("greeting/greeting.go")).toBe("go");
		expect(languageOf("web/src/App.svelte")).toBe("svelte");
		expect(languageOf("Dockerfile")).toBe("dockerfile");
		expect(languageOf("notes.unknown")).toBeUndefined();
		expect(languageOf(".env")).toBeUndefined();
	});

	it("colours every line without changing a character of it", async () => {
		const [go, unknown] = await highlighted(parseDiff(patch));

		for (const line of go.hunks[0].lines) {
			expect(line.tokens?.map((token) => token.text).join("")).toBe(line.text);
		}

		const farewell = go.hunks[0].lines.find((line) => line.text.startsWith("func"));
		expect(farewell?.tokens?.some((token) => token.tone.includes("--syntax-token-keyword"))).toBe(true);

		const comment = go.hunks[0].lines.find((line) => line.kind === "remove");
		expect(comment?.tokens?.every((token) => token.tone.includes("--syntax-token-comment"))).toBe(true);

		expect(unknown.hunks[0].lines.every((line) => line.tokens === undefined)).toBe(true);
	});
});
