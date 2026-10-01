import { describe, expect, it } from "vitest";
import { languageOf, parseDiff, segmentsOf, type DiffFile } from "./diff";
import { highlightable, highlighted, highlightLinesMax } from "./highlight.server";

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
			expect(segmentsOf(line).map((segment) => segment.text).join("")).toBe(line.text);
		}

		const farewell = go.hunks[0].lines.find((line) => line.text.startsWith("func"));
		expect(farewell && segmentsOf(farewell).some((segment) => segment.tone === "var(--syntax-token-keyword)")).toBe(true);

		const comment = go.hunks[0].lines.find((line) => line.kind === "remove");
		expect(comment && segmentsOf(comment).every((segment) => segment.tone === "var(--syntax-token-comment)")).toBe(true);

		expect(unknown.hunks[0].lines.every((line) => line.spans === undefined)).toBe(true);
	});

	it("carries each colour as two small numbers rather than a copy of the text and its colour name", async () => {
		const [go] = await highlighted(parseDiff(patch));
		const line = go.hunks[0].lines.find((held) => held.text.startsWith("func"));

		expect(line?.spans?.every((value) => Number.isInteger(value))).toBe(true);
		expect(JSON.stringify(line?.spans).length).toBeLessThan(JSON.stringify(line?.text).length * 2);
	});
});

function fileOf(path: string, lines: string[]): DiffFile {
	return parseDiff(
		[`diff --git a/${path} b/${path}`, `--- a/${path}`, `+++ b/${path}`, `@@ -0,0 +1,${lines.length} @@`]
			.concat(lines.map((line) => "+" + line))
			.join("\n")
	)[0];
}

describe("what gets highlighted", () => {
	it("leaves lockfiles, minified files and very long lines as plain text", () => {
		expect(highlightable(fileOf("bun.lock", ['{"lockfileVersion": 1}']))).toBe(false);
		expect(highlightable(fileOf("web/app.min.js", ["var a=1;"]))).toBe(false);
		expect(highlightable(fileOf("web/app.ts", ["const a = '" + "x".repeat(2000) + "';"]))).toBe(false);
		expect(highlightable(fileOf("web/app.ts", ["const a = 1;"]))).toBe(true);
	});

	it("stops highlighting once the budget for one page is spent", async () => {
		const files = Array.from({ length: 5 }, (_, index) =>
			fileOf(`src/file${index}.ts`, Array.from({ length: highlightLinesMax / 2 }, () => "const a = 1;"))
		);

		const done = await highlighted(files);
		const coloured = done.filter((file) => file.hunks[0].lines.some((line) => line.spans));

		expect(coloured).toHaveLength(2);
	});
});
