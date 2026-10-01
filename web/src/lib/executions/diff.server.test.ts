import { describe, expect, it } from "vitest";
import { diffFileInlineMax, diffInlineLines, reviewBudget } from "./diff";
import { highlighted } from "./highlight.server";
import { readPatch } from "./patch.server";

function zipped(text: string): Response {
	return new Response(new Response(text).body?.pipeThrough(new CompressionStream("gzip")));
}

const fileCount = 5000;
const linesPerFile = 10;

function largePatch(): string {
	const out: string[] = [];

	for (let file = 0; file < fileCount; file += 1) {
		const path = `src/area${file % 50}/component${file}.ts`;

		out.push(`diff --git a/${path} b/${path}`, `--- a/${path}`, `+++ b/${path}`, `@@ -1,${linesPerFile} +1,${linesPerFile} @@`);

		for (let line = 0; line < linesPerFile / 2; line += 1) {
			out.push(`-export const value${line} = "before ${file}";`, `+export const value${line} = "after ${file}";`);
		}
	}

	return out.join("\n");
}

describe("a review far larger than anything a page shows", () => {
	it("lists every file with its counts but keeps only a small, fixed amount of it", async () => {
		const read = await readPatch(zipped(largePatch()), reviewBudget);
		const files = await highlighted(read.files);

		expect(files).toHaveLength(fileCount);
		expect(files.every((file) => file.additions === linesPerFile / 2 && file.deletions === linesPerFile / 2)).toBe(true);

		const kept = files.reduce((total, file) => total + file.hunks.reduce((sum, hunk) => sum + hunk.lines.length, 0), 0);
		expect(kept).toBeLessThanOrEqual(diffInlineLines);
		expect(files.every((file) => file.hunks.reduce((sum, hunk) => sum + hunk.lines.length, 0) <= diffFileInlineMax)).toBe(true);
		expect(files.filter((file) => file.deferred).length).toBeGreaterThan(fileCount - diffInlineLines / linesPerFile - 1);

		const serialised = JSON.stringify({ files, truncated: read.truncated });
		expect(serialised.length).toBeLessThan(1_500_000);
	});

	it("reads one file out of the middle without holding the rest", async () => {
		const path = "src/area17/component2017.ts";
		const read = await readPatch(zipped(largePatch()), {
			inline: Infinity,
			perFile: Infinity,
			files: Infinity,
			only: path,
		});

		expect(read.files).toHaveLength(1);
		expect(read.files[0].path).toBe(path);
		expect(read.files[0].hunks[0].lines).toHaveLength(linesPerFile);
	});
});
